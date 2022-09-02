package core

import (
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/tenant"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileOrphanedResources(ctx reconcile.Context, k8s kubernetes.Client, stream eventsource.Stream) reconcile.Result {
	state := orhanedResourceState{
		Active:  tenant.ResourceList{},
		Deleted: tenant.ResourceList{},
	}

	handler := NewOrphanedResourceEventHandler(&state)
	res := eventsource.Replay(handler, stream.Events())
	if res.Failed() {
		ctx.Log.Error(res.Error, "failed to replay events")
		return ctx.Error(res.Error)
	}

	for _, r := range state.Deleted {
		ri, err := r.ResourceIdentifier()
		if err != nil {
			ctx.Log.Error(err, "failed to convert Resource to ResourceIdentifier")
			continue
		}

		// TODO: Make sure the resource is actually owned/created by the aeto tenant
		ctx.Log.V(1).Info("making sure orphaned resource is deleted", "nn", ri.NamespacedName.String(), "gvk", ri.GroupVersionKind.String())
		if err := k8s.DynamicDelete(ctx, ri.NamespacedName, ri.GroupVersionKind); client.IgnoreNotFound(err) != nil {
			ctx.Log.Error(err, "failed to delete orphaned resource", "nn", ri.NamespacedName.String(), "gvk", ri.GroupVersionKind.String())
		}
	}

	return ctx.Done()
}

type OrphanedResourceEventHandler struct {
	state *orhanedResourceState
}

type orhanedResourceState struct {
	DeleteAllowed bool
	Deleted       tenant.ResourceList
	Active        tenant.ResourceList
}

type orphanedResource struct {
	NamespacedName   types.NamespacedName
	GroupVersionKind schema.GroupVersionKind
}

func NewOrphanedResourceEventHandler(state *orhanedResourceState) eventsource.EventHandler {
	return &OrphanedResourceEventHandler{
		state: state,
	}
}

func (h *OrphanedResourceEventHandler) On(e eventsource.Event) {
	switch event := e.(type) {
	case *tenant.ResourceAdded:
		h.state.Active = append(h.state.Active, event.Resource)
		index, _ := h.state.Deleted.Find(event.Resource.Id)
		if index >= 0 {
			h.state.Deleted = h.state.Deleted.Remove(index)
		}
		break
	case *tenant.ResourceRemoved:
		index, r := h.state.Active.Find(event.ResourceId)
		h.state.Active = h.state.Active.Remove(index)
		h.state.Deleted = append(h.state.Deleted, *r)
		break
	case *tenant.ResourceGenererationFailed:
		h.state.DeleteAllowed = false
		break
	case *tenant.ResourceGenererationSuccessful:
		h.state.DeleteAllowed = true
		break
	}
}
