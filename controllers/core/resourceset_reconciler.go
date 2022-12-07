package core

import (
	"fmt"
	"sort"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/tenant"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileResourceSet(ctx reconcile.Context, k8s kubernetes.Client, stream eventsource.Stream) reconcile.Result {
	state := resourceSetState{
		ResourceSets: make(map[string]*corev1alpha1.ResourceSet),
	}

	handler := NewResourceSetEventHandler(&state)
	res := eventsource.Replay(handler, stream.Events())
	if res.Failed() {
		ctx.Log.Error(res.Error, "failed to replay ResourceSets from events")
		return ctx.Error(res.Error)
	}

	active := 0
	sets := make([]*corev1alpha1.ResourceSet, 0)
	for _, rs := range state.ResourceSets {
		if rs.Spec.Active {
			active++
		}
		sets = append(sets, rs)
	}
	if active > 1 {
		return ctx.Error(fmt.Errorf("replay resulted in multiple active ResourceSets (acitve=%d)", active))
	}

	// Ensures the resource set is sorted by name before deciding on which resource sets are too old
	sort.Sort(ResourceSetNameSorter(sets))

	oldSets := make([]*corev1alpha1.ResourceSet, 0)
	if len(sets) > config.Operator.MaxTenantResourceSets {
		var oldrs *corev1alpha1.ResourceSet
		oldrs, sets = sets[0], sets[1:]
		oldSets = append(oldSets, oldrs)
	}

	// Ensures the active resource set is the last to be applied
	sort.Slice(sets[:], func(i, j int) bool {
		return !sets[i].Spec.Active
	})

	ctx.Log.V(2).Info("replayed events onto ResourceSets", "count", len(state.ResourceSets), "active", active, "resource-sets", sets)

	// TODO: Decide what the behavior should be. Do we replace all resource sets, patch or apply or ?
	for _, rs := range sets {
		var existing corev1alpha1.ResourceSet
		if err := k8s.Get(ctx, rs.NamespacedName(), &existing); err != nil {
			if client.IgnoreNotFound(err) != nil {
				// Failed to fetch
				return ctx.Error(err)
			} else {
				// Not found, creating
				if err := k8s.Create(ctx, rs); err != nil {
					return ctx.Error(err)
				}
			}
		} else {
			// Updating existing
			existing.Labels = rs.Labels
			existing.Annotations = rs.Annotations
			existing.Spec = rs.Spec
			if err := k8s.Update(ctx, &existing); err != nil {
				return ctx.Error(err)
			}
		}
	}

	if len(oldSets) > 0 {
		for _, rs := range oldSets {
			var existing corev1alpha1.ResourceSet
			if err := k8s.Get(ctx, rs.NamespacedName(), &existing); err != nil {
				if client.IgnoreNotFound(err) != nil {
					ctx.Log.Error(res.Error, "failed to fetch ResourceSet", "resource-set", rs.NamespacedName().String())
					return ctx.Error(err)
				}
				// ResourceSet not found, moving on...
				continue
			} else {
				// Ensuring existing ResourceSet is inactive before deletion
				if existing.Spec.Active {
					ctx.Log.V(1).Info("skipping delete of old ResourceSet as it is currently set to active", "resource-set", rs.NamespacedName().String())
					continue
				}
			}

			ctx.Log.V(1).Info("deleting old ResourceSet", "resource-set", rs.NamespacedName().String())
			if err := k8s.Delete(ctx, rs); err != nil {
				ctx.Log.Error(err, "failed to delete old ResourceSet")
				return ctx.Error(err)
			}
		}
	}

	return ctx.Done()
}

type ResourceSetEventHandler struct {
	state *resourceSetState
}

type resourceSetState struct {
	Current      string
	Labels       map[string]string
	Annotations  map[string]string
	ResourceSets map[string]*corev1alpha1.ResourceSet
}

func NewResourceSetEventHandler(state *resourceSetState) eventsource.EventHandler {
	return &ResourceSetEventHandler{
		state: state,
	}
}

func (h *ResourceSetEventHandler) onResourceSet(name string, action func(rs *corev1alpha1.ResourceSet)) {
	rs := h.state.ResourceSets[name]
	if rs != nil {
		action(rs)
	}
}

func (h *ResourceSetEventHandler) onCurrentResourceSet(action func(rs *corev1alpha1.ResourceSet)) {
	rs := h.state.ResourceSets[h.state.Current]
	if rs != nil {
		action(rs)
	}
}

func (h *ResourceSetEventHandler) On(e eventsource.Event) {
	switch event := e.(type) {
	case *tenant.LabelsChanged:
		h.state.Labels = event.Labels
	case *tenant.AnnotationsChanged:
		h.state.Annotations = event.Annotations
	case *tenant.ResourceSetCreated:
		current := h.state.ResourceSets[h.state.Current]
		if current != nil {
			h.state.ResourceSets[event.Name] = current.DeepCopy()
		} else {
			h.state.ResourceSets[event.Name] = NewResourceSet()
		}
		h.state.Current = event.Name
		h.onCurrentResourceSet(func(rs *corev1alpha1.ResourceSet) {
			rs.ObjectMeta = metav1.ObjectMeta{
				Name:        event.Name,
				Namespace:   event.Namespace,
				Labels:      h.state.Labels,
				Annotations: h.state.Annotations,
			}
		})
	case *tenant.ResourceAdded:
		h.onCurrentResourceSet(func(rs *corev1alpha1.ResourceSet) {
			rs.Spec.Resources = append(rs.Spec.Resources, corev1alpha1.ResourceSetResource{
				Id:    event.Resource.Id,
				Order: event.Resource.Order,
				Embedded: corev1alpha1.EmbeddedResource{
					RawExtension: runtime.RawExtension{
						Raw: event.Resource.Embedded.Raw,
					},
				},
			})
		})
	case *tenant.ResourceUpdated:
		h.onCurrentResourceSet(func(rs *corev1alpha1.ResourceSet) {
			i, r := rs.Spec.Resources.Find(event.Resource.Id)
			r.Order = event.Resource.Order
			r.Embedded = corev1alpha1.EmbeddedResource{
				RawExtension: runtime.RawExtension{
					Raw: event.Resource.Embedded.Raw,
				},
			}
			rs.Spec.Resources[i] = *r
		})
	case *tenant.ResourceRemoved:
		h.onCurrentResourceSet(func(rs *corev1alpha1.ResourceSet) {
			i, _ := rs.Spec.Resources.Find(event.ResourceId)
			rs.Spec.Resources = remove(rs.Spec.Resources, i)
		})
	case *tenant.ResourceSetActivated:
		h.onResourceSet(event.Name, func(rs *corev1alpha1.ResourceSet) {
			rs.Spec.Active = true
		})
	case *tenant.ResourceSetDeactivated:
		h.onResourceSet(event.Name, func(rs *corev1alpha1.ResourceSet) {
			rs.Spec.Active = false
		})
	}

	for _, rs := range h.state.ResourceSets {
		sort.Slice(rs.Spec.Resources[:], func(i, j int) bool {
			return rs.Spec.Resources[i].Order < rs.Spec.Resources[j].Order
		})
	}
}

func NewResourceSet() *corev1alpha1.ResourceSet {
	return &corev1alpha1.ResourceSet{
		Spec: corev1alpha1.ResourceSetSpec{
			Resources: make([]corev1alpha1.ResourceSetResource, 0),
		},
	}
}

func remove(slice corev1alpha1.ResourceSetResourceList, index int) corev1alpha1.ResourceSetResourceList {
	return append(slice[:index], slice[index+1:]...)
}

// ResourceSetNameSorter sorts ResourceSet's by name.
type ResourceSetNameSorter []*corev1alpha1.ResourceSet

func (a ResourceSetNameSorter) Len() int           { return len(a) }
func (a ResourceSetNameSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ResourceSetNameSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }
