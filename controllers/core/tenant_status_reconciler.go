package core

import (
	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/tenant"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ReconcileStatus(ctx reconcile.Context, client kubernetes.Client, tenant corev1alpha1.Tenant, stream eventsource.Stream) reconcile.Result {
	handler := NewTenantStatusEventHandler(&tenant.Status)
	res := eventsource.Replay(handler, stream.Events())
	if res.Failed() {
		ctx.Log.Error(res.Error, "failed to replay Tenant status from events")
		return ctx.Error(res.Error)
	}

	rsReady := false
	if tenant.Status.ResourceSet != "" {
		nn := types.NamespacedName{
			Namespace: config.Operator.Namespace,
			Name:      tenant.Status.ResourceSet,
		}
		var rs corev1alpha1.ResourceSet
		err := client.Get(ctx, nn, &rs)
		if err != nil {
			ctx.Log.V(1).Info("failed to fetch ResourceSet, unable to check readiniess")
		} else {
			rsReadyCondition := apimeta.FindStatusCondition(rs.Status.Conditions, ConditionTypeReady)
			if rsReadyCondition != nil {
				rsReady = true
				reason := "ResourceSetReady"
				message := "ResourceSet reconciled and ready"
				if rsReadyCondition.Status != metav1.ConditionTrue {
					rsReady = false
					reason = "ResourceSetNotReady"
					message = "ResourceSet reconciled but not ready"
				}
				readyCondition := metav1.Condition{
					Type:               ConditionTypeReady,
					Status:             rsReadyCondition.Status,
					Reason:             reason,
					Message:            message,
					LastTransitionTime: rsReadyCondition.LastTransitionTime,
				}
				apimeta.SetStatusCondition(&tenant.Status.Conditions, readyCondition)
			}
		}
	}

	if err := client.UpdateStatus(ctx, &tenant); err != nil {
		return ctx.Error(err)
	}

	if !rsReady {
		return ctx.RequeueIn(15, "waiting for active ResourceSet to become ready")
	}

	return ctx.Done()
}

type TenantStatusEventHandler struct {
	state *corev1alpha1.TenantStatus
}

func NewTenantStatusEventHandler(state *corev1alpha1.TenantStatus) eventsource.EventHandler {
	state.Events = 0
	readyCondition := metav1.Condition{
		Type:    ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  "Initializing",
		Message: "Initializing Tenant",
	}
	apimeta.SetStatusCondition(&state.Conditions, readyCondition)

	return &TenantStatusEventHandler{
		state: state,
	}
}

func (h *TenantStatusEventHandler) On(e eventsource.Event) {
	// TODO: Known .status.conditions.type are: "Available", "Progressing", and "Degraded"
	// TODO: Set tenant status to ready when there are no pending events to be replayed and the current resource set has been reconciled
	// TODO: Use ObservedGeneration for conditions
	switch event := e.(type) {
	case *tenant.TenantCreated:
		reconcilingCondition := metav1.Condition{
			Type:    ConditionTypeReconciling,
			Status:  metav1.ConditionTrue,
			Reason:  "TenantCreated",
			Message: "Reconciling Tenant",
		}
		apimeta.SetStatusCondition(&h.state.Conditions, reconcilingCondition)
		readyCondition := metav1.Condition{
			Type:    ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "TenantCreated",
			Message: "Reconciling Tenant",
		}
		apimeta.SetStatusCondition(&h.state.Conditions, readyCondition)
		h.state.Phase = ConditionTypeReconciling
	case *tenant.BlueprintSet:
		h.state.Blueprint = event.Name
		break
	case *tenant.ResourceSetNameChanged:
		h.state.ResourceSet = event.Name
		break
	case *tenant.TenantDeleted:
		reconcilingCondition := metav1.Condition{
			Type:    ConditionTypeReconciling,
			Status:  metav1.ConditionFalse,
			Reason:  "TenantDeleted",
			Message: "Performing cleanup",
		}
		apimeta.SetStatusCondition(&h.state.Conditions, reconcilingCondition)
		readyCondition := metav1.Condition{
			Type:    ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  "TenantDeleted",
			Message: "Performing cleanup",
		}
		apimeta.SetStatusCondition(&h.state.Conditions, readyCondition)
		terminatingCondition := metav1.Condition{
			Type:    ConditionTypeTerminating,
			Status:  metav1.ConditionTrue,
			Reason:  "TenantDeleted",
			Message: "Performing cleanup",
		}
		apimeta.SetStatusCondition(&h.state.Conditions, terminatingCondition)
		h.state.Phase = ConditionTypeTerminating
		break
	}

	h.state.Events++
}
