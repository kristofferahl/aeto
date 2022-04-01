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

package event

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventv1alpha1 "github.com/kristofferahl/aeto/apis/event/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
)

// EventStreamChunkReconciler reconciles a EventStreamChunk object
type EventStreamChunkReconciler struct {
	kubernetes.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=event.aeto.net,resources=eventstreamchunks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=event.aeto.net,resources=eventstreamchunks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=event.aeto.net,resources=eventstreamchunks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EventStreamChunk object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *EventStreamChunkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EventStreamChunkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&eventv1alpha1.EventStreamChunk{}).
		Complete(r)
}
