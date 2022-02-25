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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/dynamic"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

const (
	TenantFinalizerName = "tenant.core.aeto.net/finalizer"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Dynamic  dynamic.Clients
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

	_, err := r.getTenant(rctx)
	if err != nil {
		rctx.Log.Info("Tenant not found")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	rctx.Log.V(1).Info("found Tenant")

	return ctrl.Result{}, nil
}

func (r *TenantReconciler) getTenant(ctx reconcile.Context) (corev1alpha1.Tenant, error) {
	var tenant corev1alpha1.Tenant
	if err := r.Get(ctx.Context, ctx.Request.NamespacedName, &tenant); err != nil {
		return corev1alpha1.Tenant{}, err
	}
	return tenant, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Tenant{}).
		Complete(r)
}
