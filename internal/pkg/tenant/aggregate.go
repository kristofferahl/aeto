package tenant

import (
	"fmt"
	"reflect"

	"github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
)

type TenantAggregate struct {
	root  eventsource.AggregateRoot
	state State
}

type State struct {
	TenantName        string
	TenantNamespace   string
	TenantDisplayName string

	BlueprintName string

	Labels      map[string]string
	Annotations map[string]string

	ResourceGenerationFailed bool
	ResourceGenerationSum    string

	ResourceSetVersion int
	ResourceSetName    string
	Resources          ResourceList

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
	a := NewTenant(stream.Id)
	a.root.
		LoadFromHistoricalEvents(stream).
		WithVersion(int64(len(stream.Commits))) // TODO: This could be a bad idea, better to use commit sequence number?
	return a
}

func (a *TenantAggregate) Initialize(name string, namespace string) {
	a.root.Apply(&TenantCreated{Name: name, Namespace: namespace})
}

func (a *TenantAggregate) SetDisplayName(name string) {
	if a.state.TenantDisplayName != name {
		a.root.Apply(&TenantDisplayNameSet{Name: name})
	}
}

func (a *TenantAggregate) SetBlueprintName(name string) {
	if a.state.BlueprintName != name {
		a.root.Apply(&BlueprintSet{Name: name})
	}
}

func (a *TenantAggregate) From(g ResourceGenerator, t v1alpha1.Tenant, b v1alpha1.Blueprint) error {
	commonLables := b.CommonLabels(t)
	commonAnnotations := b.CommonAnnotations(t)
	if !reflect.DeepEqual(a.state.Labels, commonLables) {
		a.root.Apply(&LabelsChanged{Labels: commonLables})
	}

	if !reflect.DeepEqual(a.state.Annotations, commonAnnotations) {
		a.root.Apply(&AnnotationsChanged{Annotations: commonAnnotations})
	}

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

	if a.state.BlueprintName != b.Name {
		a.root.Apply(&BlueprintSet{Name: b.Name})
	}

	if resourcesChanged {
		a.root.Apply(&ResourceSetVersionChanged{Version: a.state.ResourceSetVersion + 1})
		a.root.Apply(&ResourceSetNameChanged{Name: fmt.Sprintf("rs-%s-%06d", a.state.TenantName, a.state.ResourceSetVersion)})
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

	if a.state.ResourceSetActive[a.state.ResourceSetName] == false {
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
	commit := eventsource.Commit{}
	a.root.CommitEvents(func(e eventsource.Event) {
		commit.Events = append(commit.Events, e)
	})
	commit.Id = fmt.Sprintf("%s-stream-chunk-%06d", a.root.Id(), a.root.Version())
	return commit
}

func (s *State) On(e eventsource.Event) {
	switch event := e.(type) {
	case *TenantCreated:
		s.TenantName = event.Name
		s.TenantNamespace = event.Namespace
		break
	case *TenantDisplayNameSet:
		s.TenantDisplayName = event.Name
		break
	case *BlueprintSet:
		s.BlueprintName = event.Name
		break
	case *LabelsChanged:
		s.Labels = event.Labels
		break
	case *AnnotationsChanged:
		s.Annotations = event.Annotations
		break
	case *ResourceGenererationFailed:
		s.ResourceGenerationFailed = true
		s.ResourceGenerationSum = event.Sum
		break
	case *ResourceGenererationSuccessful:
		s.ResourceGenerationFailed = false
		s.ResourceGenerationSum = event.Sum
		break
	case *ResourceSetVersionChanged:
		s.ResourceSetVersion = event.Version
		break
	case *ResourceSetNameChanged:
		s.ResourceSetName = event.Name
		break
	case *ResourceAdded:
		s.Resources = append(s.Resources, event.Resource)
		break
	case *ResourceUpdated:
		index, _ := s.Resources.Find(event.Resource.Id)
		s.Resources[index] = *&event.Resource
		break
	case *ResourceRemoved:
		index, _ := s.Resources.Find(event.ResourceId)
		s.Resources = s.Resources.Remove(index)
		break
	case *ResourceSetActivated:
		s.ResourceSetActive[event.Name] = true
		break
	case *ResourceSetDeactivated:
		s.ResourceSetActive[event.Name] = false
		break
	case *TenantDeleted:
		s.Deleted = true
		break
	}
}
