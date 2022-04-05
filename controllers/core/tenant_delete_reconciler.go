package core

import (
	"fmt"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/tenant"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileDelete(ctx reconcile.Context, k8s kubernetes.Client, store eventsource.Repository, stream eventsource.Stream) reconcile.Result {
	state := deleteState{
		ResourceSets: make([]string, 0),
	}

	handler := NewDeleteEventHandler(&state)
	res := eventsource.Replay(handler, stream.Events())
	if res.Failed() {
		ctx.Log.Error(res.Error, "failed to replay delete instructions from events")
		return ctx.Error(res.Error)
	}

	deletedResourceSets := 0
	if len(state.ResourceSets) > 0 {
		for _, rs := range state.ResourceSets {
			nn := types.NamespacedName{
				Namespace: config.Operator.Namespace,
				Name:      rs,
			}
			var existing corev1alpha1.ResourceSet
			if err := k8s.Get(ctx, nn, &existing); err != nil {
				if client.IgnoreNotFound(err) != nil {
					// Failed to fetch
					continue
				} else {
					// Already deleted, all good...
					deletedResourceSets++
				}
			} else {
				if err := k8s.Delete(ctx, &corev1alpha1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: nn.Namespace,
						Name:      nn.Name,
					},
				}); client.IgnoreNotFound(err) != nil {
					continue
				}
			}
		}
	}

	if deletedResourceSets != len(state.ResourceSets) {
		return ctx.RequeueIn(15, fmt.Sprintf("%d out of %d ResourceSets deleted", deletedResourceSets, len(state.ResourceSets)))
	}

	err := store.Delete(stream)
	if err != nil {
		ctx.Log.Error(err, "failed to delete EventStoreChunk(s)")
		return ctx.Error(err)
	}

	return ctx.Done()
}

type DeleteEventHandler struct {
	state *deleteState
}

type deleteState struct {
	ResourceSets []string
}

func NewDeleteEventHandler(state *deleteState) eventsource.EventHandler {
	return &DeleteEventHandler{
		state: state,
	}
}

func (h *DeleteEventHandler) On(e eventsource.Event) {
	switch event := e.(type) {
	case *tenant.ResourceSetCreated:
		h.state.ResourceSets = append(h.state.ResourceSets, event.Name)
		break
	}
}
