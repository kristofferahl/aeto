/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package core

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	eventv1alpha1 "github.com/kristofferahl/aeto/apis/event/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/eventstore"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	domain "github.com/kristofferahl/aeto/internal/pkg/tenant"
)

const (
	TenantFinalizerName = "tenant.core.aeto.net/finalizer"
)

var (
	serializer = eventstore.NewSerializer(domain.Events()...)
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	kubernetes.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=core.aeto.net,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.aeto.net,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.aeto.net,resources=tenants/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Tenant object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("tenant", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	var tenant corev1alpha1.Tenant
	if err := r.Get(rctx, req.NamespacedName, &tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := reconcile.NewGenericFinalizer(TenantFinalizerName, func(c reconcile.Context) reconcile.Result {
		streamId := eventstore.StreamId(req.NamespacedName.String())
		store := eventstore.New(r.Client.GetClient(), rctx.Log, rctx.Context, serializer)
		stream, err := store.Get(streamId)
		if err != nil {
			return rctx.Error(err)
		}
		if stream.Length() > 0 {
			results := reconcile.ResultList{}

			results = append(results, ReconcileStatus(rctx, r.Client, tenant, stream))
			results = append(results, ReconcileOrphanedResources(rctx, r.Client, stream))
			results = append(results, ReconcileDelete(rctx, r.Client, store, stream))

			if results.AllDone() {
				return rctx.Done()
			}

			rctx.Log.V(1).Info("event stream found, loading Tenant aggregate from history")
			t := domain.NewTenantFromEvents(stream)

			t.Delete()

			_, err := store.Save(t)
			if err != nil {
				return rctx.Error(err)
			} else {
				return rctx.RequeueIn(5, "events might need to be processed in the finalizer")
			}
		}
		return rctx.Done()
	})
	res, err := reconcile.WithFinalizer(r.Client.GetClient(), rctx, &tenant, finalizer)
	if reconcile.FinalizerInProgress(res, err) {
		return *res, err
	}

	blueprintRef := types.NamespacedName{
		Namespace: config.Operator.Namespace,
		Name:      tenant.Blueprint(),
	}
	var blueprint corev1alpha1.Blueprint
	if err := r.Get(rctx, blueprintRef, &blueprint); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	streamId := eventstore.StreamId(req.NamespacedName.String())
	store := eventstore.New(r.Client.GetClient(), rctx.Log, rctx.Context, serializer)
	stream, err := store.Get(streamId)
	if err != nil {
		return ctrl.Result{}, err
	}

	results := reconcile.ResultList{}

	if stream.Length() == 0 {
		sr := ReconcileStatus(rctx, r.Client, tenant, stream)
		results = append(results, sr)

		rctx.Log.V(1).Info("no events, creating new Tenant aggregate")
		t := domain.NewTenant(streamId)

		t.Create(tenant.Name, tenant.Namespace)
		t.SetDisplayName(tenant.Spec.Name)
		t.SetBlueprint(tenant, blueprint)

		events, err := store.Save(t)
		if err != nil {
			results = append(results, rctx.Error(err))
		} else if events > 0 {
			results = append(results, rctx.RequeueIn(5, "new events needs processing by the controller"))
		}
	} else {
		rctx.Log.V(1).Info("event stream found, loading Tenant aggregate from history")
		t := domain.NewTenantFromEvents(stream)

		results = append(results, ReconcileResourceSet(rctx, r.Client, stream))
		results = append(results, ReconcileOrphanedResources(rctx, r.Client, stream))
		results = append(results, ReconcileRequeueRequest(rctx, stream))
		results = append(results, ReconcileStatus(rctx, r.Client, tenant, stream))

		t.SetDisplayName(tenant.Spec.Name)
		t.SetBlueprint(tenant, blueprint)

		generator := domain.NewResourceGenerator(rctx, domain.ResourceGeneratoreServices{Client: r.Client})

		err = t.GenerateResources(generator, tenant, blueprint)
		if err != nil {
			rctx.Log.Error(err, "failed to generate events from Blueprint")
			results = append(results, rctx.Error(err))
		}

		events, err := store.Save(t)
		if err != nil {
			results = append(results, rctx.Error(err))
		} else if events > 0 {
			results = append(results, rctx.RequeueIn(5, "new events needs processing by the controller"))
		}
	}

	return rctx.Complete(results...)
}

func (r *TenantReconciler) getBlueprint(ctx reconcile.Context, nn types.NamespacedName) (corev1alpha1.Blueprint, error) {
	var blueprint corev1alpha1.Blueprint
	if err := r.Get(ctx, nn, &blueprint); err != nil {
		return corev1alpha1.Blueprint{}, err
	}
	return blueprint, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &eventv1alpha1.EventStreamChunk{}, eventstore.StreamIdFieldIndexKey, func(o client.Object) []string {
		chunk := o.(*eventv1alpha1.EventStreamChunk)
		if chunk.Spec.StreamId == "" {
			return nil
		}
		return []string{chunk.Spec.StreamId}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Tenant{}).
		Complete(r)
}
