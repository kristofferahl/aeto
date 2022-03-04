package core

import (
	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/tenant"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileStatus(ctx reconcile.Context, client client.Client, tenant corev1alpha1.Tenant, stream eventsource.Stream) reconcile.Result {
	handler := NewTenantStatusEventHandler(&tenant.Status)
	res := eventsource.Replay(handler, stream.Events())
	if res.Failed() {
		ctx.Log.Error(res.Error, "Failed to replay Tenant status from events")
		return ctx.Error(res.Error)
	}

	ctx.Log.V(1).Info("updating Tenant status")
	if err := client.Status().Update(ctx.Context, &tenant); err != nil {
		ctx.Log.Error(err, "failed to update Tenant status")
		return ctx.Error(err)
	}

	return ctx.Done()
}

type TenantStatusEventHandler struct {
	state *corev1alpha1.TenantStatus
}

func NewTenantStatusEventHandler(state *corev1alpha1.TenantStatus) eventsource.EventHandler {
	return &TenantStatusEventHandler{
		state: state,
	}
}

func (h *TenantStatusEventHandler) On(e eventsource.Event) {
	switch event := e.(type) {
	case *tenant.BlueprintSet:
		h.state.Blueprint = event.Name
		break
	}
}
