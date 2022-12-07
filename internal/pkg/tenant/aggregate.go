package tenant

import (
	"fmt"
	"reflect"

	"github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
)

type TenantAggregate struct {
	root  eventsource.AggregateRoot
	state State
}

type State struct {
	TenantName      string
	TenantNamespace string
	TenantFullName  string

	BlueprintName      string
	BlueprintNamespace string

	Labels      map[string]string
	Annotations map[string]string

	TenantPrefixedName      string
	TenantPrefixedNamespace string

	ResourceGenerationFailed bool
	ResourceGenerationSum    string

	ResourceSetVersion   int
	ResourceSetName      string
	ResourceSetNamespace string
	Resources            ResourceList

	ResourceSetActive map[string]bool

	Deleted bool
}

func NewTenant(id string) *TenantAggregate {
	a := &TenantAggregate{
		root: eventsource.AggregateRoot{},
		state: State{
			ResourceSetActive: make(map[string]bool),
		},
	}
	a.root.
		WithId(id).
		WithVersion(0).
		WithHandler(&a.state)
	return a
}

func NewTenantFromEvents(stream eventsource.Stream) *TenantAggregate {
	a := NewTenant(stream.Id())
	a.root.LoadFromHistoricalEvents(stream)
	return a
}

func (a *TenantAggregate) Create(name string, namespace string) {
	a.root.Apply(&TenantCreated{Name: name, Namespace: namespace})
}

func (a *TenantAggregate) SetFullName(name string) {
	if a.state.TenantFullName != name {
		a.root.Apply(&TenantFullNameSet{Name: name})
	}
}

func (a *TenantAggregate) SetBlueprint(tenant v1alpha1.Tenant, blueprint v1alpha1.Blueprint) {
	if a.state.BlueprintName != blueprint.Name || a.state.BlueprintNamespace != blueprint.Namespace {
		a.root.Apply(&BlueprintSet{Name: blueprint.Name, Namespace: blueprint.Namespace})
	}

	name := blueprint.Spec.ResourceNamePrefix + a.state.TenantName
	namespace := blueprint.Spec.ResourceNamePrefix + a.state.TenantName
	if a.state.TenantPrefixedName != name || a.state.TenantPrefixedNamespace != namespace {
		a.root.Apply(&ResourceNamespaceNameChanged{Name: name, Namespace: namespace})
	}

	commonLables := blueprint.CommonLabels(tenant)
	commonAnnotations := blueprint.CommonAnnotations(tenant)

	if !reflect.DeepEqual(a.state.Labels, commonLables) {
		a.root.Apply(&LabelsChanged{Labels: commonLables})
	}

	if !reflect.DeepEqual(a.state.Annotations, commonAnnotations) {
		a.root.Apply(&AnnotationsChanged{Annotations: commonAnnotations})
	}
}

func (a *TenantAggregate) GenerateResources(g ResourceGenerator, t v1alpha1.Tenant, b v1alpha1.Blueprint) error {
	res, err := g.Generate(a.state, b)
	resourcesChanged := a.state.ResourceGenerationSum != res.Sum
	if err != nil {
		if !a.state.ResourceGenerationFailed || resourcesChanged {
			a.root.Apply(&ResourceGenererationFailed{Sum: res.Sum})
		}
		if len(res.ResourceGroups) == 0 {
			return err
		}
	} else {
		if a.state.ResourceGenerationFailed || resourcesChanged {
			a.root.Apply(&ResourceGenererationSuccessful{Sum: res.Sum})
		}
	}

	if resourcesChanged {
		a.root.Apply(&ResourceSetVersionChanged{Version: a.state.ResourceSetVersion + 1})
		a.root.Apply(&ResourceSetCreated{Name: fmt.Sprintf("rs-%s-%06d", a.state.TenantName, a.state.ResourceSetVersion), Namespace: config.Operator.Namespace})
	}

	for _, rg := range res.ResourceGroups {
		rg := rg
		for _, r := range rg.Resources {
			r := r
			_, existing := a.state.Resources.Find(r.Id)
			if existing != nil {
				if existing.Sum != r.Sum || existing.Order != r.Order {
					a.root.Apply(&ResourceUpdated{
						Resource: r,
					})
				}
			} else {
				a.root.Apply(&ResourceAdded{
					Resource: r,
				})
			}
		}
	}

	for _, sr := range a.state.Resources {
		_, found := res.ResourceGroups.Resources().Find(sr.Id)
		if found == nil {
			a.root.Apply(&ResourceRemoved{
				ResourceId: sr.Id,
			})
		}
	}

	for rsn, active := range a.state.ResourceSetActive {
		if active && rsn != a.state.ResourceSetName {
			a.root.Apply(&ResourceSetDeactivated{Name: rsn})
		}
	}

	if !a.state.ResourceSetActive[a.state.ResourceSetName] {
		a.root.Apply(&ResourceSetActivated{Name: a.state.ResourceSetName})
	}

	return nil
}

func (a *TenantAggregate) Delete() {
	if !a.state.Deleted {
		a.root.Apply(&TenantDeleted{})
	}
}

func (a *TenantAggregate) Id() string {
	return a.root.Id()
}

func (a *TenantAggregate) Version() int64 {
	return a.root.Version()
}

func (a *TenantAggregate) Commit() eventsource.Commit {
	next := a.root.Version() + 1
	commit := eventsource.NewCommit(fmt.Sprintf("%s-stream-chunk-%06d", a.root.Id(), next), next)
	a.root.CommitEvents(next, func(e eventsource.Event) {
		commit.Append(e)
	})
	return *commit
}

func (s *State) On(e eventsource.Event) {
	switch event := e.(type) {
	case *TenantCreated:
		s.TenantName = event.Name
		s.TenantNamespace = event.Namespace
	case *TenantDisplayNameSet:
		s.TenantFullName = event.Name
	case *TenantFullNameSet:
		s.TenantFullName = event.Name
	case *BlueprintSet:
		s.BlueprintName = event.Name
		s.BlueprintNamespace = event.Namespace
	case *LabelsChanged:
		s.Labels = event.Labels
	case *AnnotationsChanged:
		s.Annotations = event.Annotations
	case *ResourceNamespaceNameChanged:
		s.TenantPrefixedName = event.Name
		s.TenantPrefixedNamespace = event.Namespace
	case *ResourceGenererationFailed:
		s.ResourceGenerationFailed = true
		s.ResourceGenerationSum = event.Sum
	case *ResourceGenererationSuccessful:
		s.ResourceGenerationFailed = false
		s.ResourceGenerationSum = event.Sum
	case *ResourceSetVersionChanged:
		s.ResourceSetVersion = event.Version
	case *ResourceSetCreated:
		s.ResourceSetName = event.Name
		s.ResourceSetNamespace = event.Namespace
	case *ResourceAdded:
		s.Resources = append(s.Resources, event.Resource)
	case *ResourceUpdated:
		index, _ := s.Resources.Find(event.Resource.Id)
		s.Resources[index] = event.Resource
	case *ResourceRemoved:
		index, _ := s.Resources.Find(event.ResourceId)
		s.Resources = s.Resources.Remove(index)
	case *ResourceSetActivated:
		s.ResourceSetActive[event.Name] = true
	case *ResourceSetDeactivated:
		s.ResourceSetActive[event.Name] = false
	case *TenantDeleted:
		s.Deleted = true
	}
}
