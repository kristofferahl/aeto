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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/PaesslerAG/jsonpath"
	corev1alpha1 "github.com/kristofferahl/aeto/apis/core/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/convert"
	"github.com/kristofferahl/aeto/internal/pkg/dynamic"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

const (
	FinalizerName = "resourceset.core.aeto.net/finalizer"
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

	finalizer := reconcile.NewGenericFinalizer(FinalizerName, func(c reconcile.Context) reconcile.Result {
		resourceSet := resourceSet
		return r.reconcileDelete(c, resourceSet)
	})
	res, err := reconcile.WithFinalizer(r.Client, rctx, &resourceSet, finalizer)
	if res != nil || err != nil {
		rctx.Log.V(1).Info("returning finalizer results for ResourceSet", "resource-set", resourceSet.NamespacedName(), "res", res, "error", err)
		return *res, err
	}

	results := make([]reconcile.Result, 0)

	for _, group := range resourceSet.Spec.Groups {
		for _, resource := range group.Resources {
			result := r.applyResource(rctx, resourceSet, group, resource)
			results = append(results, result)
		}
	}

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
			if err := r.Dynamic.Delete(ctx, types.NamespacedName{
				Name:      unstructured.GetName(),
				Namespace: unstructured.GetNamespace(),
			}, unstructured.GroupVersionKind()); err != nil {
				results = append(results, ctx.Error(err))
				continue
			}

			results = append(results, ctx.Done())
		}
	}

	if results.Success() {
		ctx.Log.V(1).Info("all resources belonging to ResourceSet deleted")
		return ctx.Done()
	}

	ctx.Log.Info("failed to delete one ore more resources belonging to ResourceSet, requeue")
	return ctx.RequeueIn(15)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.ResourceSet{}).
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
