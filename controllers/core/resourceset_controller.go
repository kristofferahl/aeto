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
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/PaesslerAG/jsonpath"
	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/convert"
	"github.com/kristofferahl/aeto/internal/pkg/dynamic"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/util"
)

const (
	ResourceSetFinalizerName = "resourceset.core.aeto.net/finalizer"
)

// ResourceSetReconciler reconciles a ResourceSet object
type ResourceSetReconciler struct {
	client.Client
	Dynamic dynamic.Clients
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=core.aeto.net,resources=resourcesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.aeto.net,resources=resourcesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.aeto.net,resources=resourcesets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ResourceSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ResourceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("resourceset", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	resourceSet, err := r.getResourceSet(rctx, req)
	if err != nil {
		rctx.Log.Info("ResourceSet not found")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	rctx.Log.V(1).Info("found ResourceSet")

	finalizer := reconcile.NewGenericFinalizer(ResourceSetFinalizerName, func(c reconcile.Context) reconcile.Result {
		resourceSet := resourceSet
		if resourceSet.Status.Phase != corev1alpha1.ResourceSetTerminating {
			res := r.updateStatus(c, resourceSet, corev1alpha1.ResourceSetTerminating)
			if res.Error() {
				return res
			}
			return c.RequeueIn(5)
		}
		return r.reconcileDelete(c, resourceSet)
	})
	res, err := reconcile.WithFinalizer(r.Client, rctx, &resourceSet, finalizer)
	if reconcile.FinalizerInProgress(res, err) {
		return *res, err
	}

	results := reconcile.ResultList{}

	for _, group := range resourceSet.Spec.Groups {
		for _, resource := range group.Resources {
			result := r.applyResource(rctx, resourceSet, group, resource)
			results = append(results, result)
		}
	}

	results = append(results, r.updateStatus(rctx, resourceSet, corev1alpha1.ResourceSetReconciling))

	return rctx.Complete(results...)
}

func (r *ResourceSetReconciler) getResourceSet(ctx reconcile.Context, req ctrl.Request) (corev1alpha1.ResourceSet, error) {
	var rs corev1alpha1.ResourceSet
	if err := r.Get(ctx.Context, req.NamespacedName, &rs); err != nil {
		return corev1alpha1.ResourceSet{}, err
	}
	return rs, nil
}

func (r *ResourceSetReconciler) applyResource(ctx reconcile.Context, resourceSet corev1alpha1.ResourceSet, group corev1alpha1.ResourceSetResourceGroup, resource corev1alpha1.EmbeddedResource) reconcile.Result {
	b, err := json.Marshal(resource)
	if err != nil {
		return ctx.Error(err)
	}

	manifest := string(b)

	v := interface{}(nil)
	json.Unmarshal(b, &v)

	resourceRef := types.NamespacedName{}

	namespace, err := jsonpath.Get("$.metadata.namespace", v)
	if err == nil {
		resourceRef.Namespace = namespace.(string)
	}

	name, err := jsonpath.Get("$.metadata.name", v)
	if err == nil {
		resourceRef.Name = name.(string)
	}

	err = r.Dynamic.Apply(ctx, resourceRef, manifest)
	if err != nil {
		ctx.Log.Error(err, "failed to apply resource from ResourceSet", "group", group.Name, "template", group.SourceTemplate, "resource", resource)
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *ResourceSetReconciler) reconcileDelete(ctx reconcile.Context, resourceSet corev1alpha1.ResourceSet) reconcile.Result {
	results := reconcile.ResultList{}

	for _, group := range reverseResourceGroupList(resourceSet.Spec.Groups) {
		for _, resource := range reverseEmbeddedResourceList(group.Resources) {
			ctx.Log.V(1).Info("deleting resource belonging to ResourceSet", "group", group.Name, "template", group.SourceTemplate, "resource", resource)

			unstructuredList, err := convert.YamlToUnstructuredSlice(string(resource.Raw))
			if err != nil {
				results = append(results, ctx.Error(err))
				continue
			}

			if len(unstructuredList) != 1 {
				results = append(results, ctx.Error(fmt.Errorf("expected exactly 1 unstructured resource, got %d", len(unstructuredList))))
				continue
			}

			unstructured := unstructuredList[0]
			nn := types.NamespacedName{
				Name:      unstructured.GetName(),
				Namespace: unstructured.GetNamespace(),
			}

			if err := r.Dynamic.Delete(ctx, nn, unstructured.GroupVersionKind()); err != nil {
				results = append(results, ctx.Error(err))
				continue
			}

			unstructured, err = r.Dynamic.Get(ctx, nn, unstructured.GroupVersionKind())
			if err != nil {
				results = append(results, ctx.Error(err))
				continue
			}

			if unstructured != nil {
				ctx.Log.V(1).Info("resource is still being terminated, requeue", "resource", nn, "gvk", unstructured.GroupVersionKind())
				results = append(results, ctx.RequeueIn(5))
				continue
			}

			results = append(results, ctx.Done())
		}
	}

	for _, res := range results {
		if res.Requeue() {
			ctx.Log.Info("one ore more resources belonging to ResourceSet are still being terminated, requeue")
			return res
		}
	}

	ctx.Log.V(1).Info("all resources belonging to ResourceSet deleted")
	return ctx.Done()
}

func (r *ResourceSetReconciler) updateStatus(ctx reconcile.Context, resourceSet corev1alpha1.ResourceSet, phase corev1alpha1.ResourceSetPhase) reconcile.Result {
	var rs corev1alpha1.ResourceSet
	if err := r.Get(ctx.Context, resourceSet.NamespacedName(), &rs); err != nil {
		return ctx.Error(err)
	}

	rs.Status.Phase = phase
	rs.Status.ObservedGeneration = rs.GetGeneration()
	rs.Status.ResourceVersion = rs.GetResourceVersion()

	if util.AsSha256(resourceSet.Status) != util.AsSha256(rs.Status) {
		ctx.Log.V(1).Info("updating ResourceSet status")
		if err := r.Status().Update(ctx.Context, &rs); err != nil {
			ctx.Log.Error(err, "failed to update ResourceSet status")
			return ctx.Error(err)
		}
	}

	return ctx.Done()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.ResourceSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func reverseResourceGroupList(s []corev1alpha1.ResourceSetResourceGroup) []corev1alpha1.ResourceSetResourceGroup {
	a := make([]corev1alpha1.ResourceSetResourceGroup, len(s))
	copy(a, s)

	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}

	return a
}

func reverseEmbeddedResourceList(s []corev1alpha1.EmbeddedResource) []corev1alpha1.EmbeddedResource {
	a := make([]corev1alpha1.EmbeddedResource, len(s))
	copy(a, s)

	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}

	return a
}
