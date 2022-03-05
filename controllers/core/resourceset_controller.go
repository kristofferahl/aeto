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
	"strings"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		if !resourceSet.Spec.Active {
			rctx.Log.Info("ResourceSet inactive, skipping cleanup before delete")
			return c.Done()
		}
		if resourceSet.Status.Phase != corev1alpha1.ResourceSetTerminating {
			res := r.updateStatus(c, resourceSet, corev1alpha1.ResourceSetTerminating, false)
			if res.Error() {
				return res
			}
			return c.RequeueIn(5, "status updated, terminating")
		}
		return r.reconcileDelete(c, resourceSet)
	})
	res, err := reconcile.WithFinalizer(r.Client, rctx, &resourceSet, finalizer)
	if reconcile.FinalizerInProgress(res, err) {
		return *res, err
	}

	results := reconcile.ResultList{}

	if resourceSet.Spec.Active {
		for _, resource := range resourceSet.Spec.Resources {
			result := r.applyResource(rctx, resource.Embedded)
			results = append(results, result)
		}
	} else {
		rctx.Log.Info("ResourceSet inactive, skipping reconcile of resources")
	}

	results = append(results, r.updateStatus(rctx, resourceSet, corev1alpha1.ResourceSetReconciling, results.AllDone()))

	return rctx.Complete(results...)
}

func (r *ResourceSetReconciler) getResourceSet(ctx reconcile.Context, req ctrl.Request) (corev1alpha1.ResourceSet, error) {
	var rs corev1alpha1.ResourceSet
	if err := r.Get(ctx.Context, req.NamespacedName, &rs); err != nil {
		return corev1alpha1.ResourceSet{}, err
	}
	return rs, nil
}

func (r *ResourceSetReconciler) applyResource(ctx reconcile.Context, resource corev1alpha1.EmbeddedResource) reconcile.Result {
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
		ctx.Log.Error(err, "failed to apply resource from ResourceSet", "resource", resource)
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *ResourceSetReconciler) reconcileDelete(ctx reconcile.Context, resourceSet corev1alpha1.ResourceSet) reconcile.Result {
	results := reconcile.ResultList{}

	for _, resource := range reverseResourceList(resourceSet.Spec.Resources) {
		ctx.Log.V(1).Info("deleting resource belonging to ResourceSet", "id", resource.Id, "resource", resource.Embedded)

		unstructuredList, err := convert.YamlToUnstructuredSlice(string(resource.Embedded.Raw))
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
			results = append(results, ctx.RequeueIn(5, fmt.Sprintf("resource is still being terminated (%s %s)", nn, unstructured.GroupVersionKind())))
			continue
		}

		results = append(results, ctx.Done())
	}

	for _, res := range results {
		if res.RequiresRequeue() {
			ctx.Log.V(1).Info("one ore more resources belonging to the ResourceSet are being terminated")
			return res
		}
	}

	ctx.Log.V(1).Info("all resources belonging to the ResourceSet have been deleted")
	return ctx.Done()
}

func (r *ResourceSetReconciler) checkResourceReady(ctx reconcile.Context, resource corev1alpha1.EmbeddedResource) bool {
	ri, err := convert.RawExtensionToResourceIdentifier(resource.RawExtension)
	if err != nil {
		ctx.Log.Error(err, "failed to convert resource to resource identifier")
		return false
	}

	ur, err := r.Dynamic.Get(ctx, ri.NamespacedName, ri.GroupVersionKind)
	if err != nil {
		return false
	}

	status, found, err := unstructured.NestedMap(ur.Object, "status")
	if err != nil || !found {
		return true
	}

	ready, err := jsonpath.Get("$.ready", status)
	ctx.Log.V(1).Info("checking status ready field", "ready", ready, "nn", ri.NamespacedName.String(), "gvk", ri.GroupVersionKind)
	if err == nil {
		rs := strings.ToLower(fmt.Sprintf("%s", ready))
		return rs != "false"
	}

	readyCondition, err := jsonpath.Get("$.conditions[?(@.type == \"Ready\")].status", status)
	ctx.Log.V(1).Info("checking status ready condition", "ready", readyCondition, "nn", ri.NamespacedName.String(), "gvk", ri.GroupVersionKind)
	if err == nil {
		results := readyCondition.([]interface{})
		if len(results) == 1 {
			return strings.ToLower(fmt.Sprintf("%s", results[0])) == "true"
		}
	}

	return true
}

func (r *ResourceSetReconciler) updateStatus(ctx reconcile.Context, resourceSet corev1alpha1.ResourceSet, phase corev1alpha1.ResourceSetPhase, resourcesApplied bool) reconcile.Result {
	resourceSet.Status.Phase = phase
	resourceSet.Status.ObservedGeneration = resourceSet.GetGeneration()
	resourceSet.Status.ResourceVersion = resourceSet.GetResourceVersion()

	if !resourceSet.Spec.Active && resourceSet.Status.Phase == corev1alpha1.ResourceSetReconciling {
		resourceSet.Status.Phase = corev1alpha1.ResourceSetPaused
	}

	active := metav1.ConditionFalse
	if resourceSet.Spec.Active {
		active = metav1.ConditionTrue
	}
	activeCondition := metav1.Condition{
		Type:    ConditionTypeActive,
		Status:  active,
		Reason:  string(resourceSet.Status.Phase),
		Message: "",
	}
	apimeta.SetStatusCondition(&resourceSet.Status.Conditions, activeCondition)

	readyReason := string(resourceSet.Status.Phase)
	readyStatus := metav1.ConditionFalse
	readyMsg := ""

	switch resourceSet.Status.Phase {
	case corev1alpha1.ResourceSetReconciling:
		desiredCount := len(resourceSet.Spec.Resources)
		readyCount := 0
		for _, resource := range resourceSet.Spec.Resources {
			if ready := r.checkResourceReady(ctx, resource.Embedded); ready {
				readyCount++
			}
		}
		ready := readyCount == desiredCount
		readyStatus = metav1.ConditionFalse
		if resourcesApplied && !ready {
			readyReason = "ResourcesNotReady"
		}
		if resourcesApplied && ready {
			readyStatus = metav1.ConditionTrue
			readyReason = "ResourcesReady"
		}
		readyMsg = fmt.Sprintf("%d/%d", readyCount, desiredCount)
		break
	case corev1alpha1.ResourceSetPaused:
		readyStatus = metav1.ConditionUnknown
		break
	case corev1alpha1.ResourceSetTerminating:
		readyStatus = metav1.ConditionFalse
		break
	}

	condition := metav1.Condition{
		Type:    ConditionTypeReady,
		Status:  readyStatus,
		Reason:  readyReason,
		Message: readyMsg,
	}
	apimeta.SetStatusCondition(&resourceSet.Status.Conditions, condition)

	ctx.Log.V(1).Info("updating ResourceSet status")
	if err := r.Status().Update(ctx.Context, &resourceSet); err != nil {
		ctx.Log.Error(err, "failed to update ResourceSet status")
		return ctx.Error(err)
	}

	return ctx.Done()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.ResourceSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func reverseResourceList(s []corev1alpha1.ResourceSetResource) []corev1alpha1.ResourceSetResource {
	a := make([]corev1alpha1.ResourceSetResource, len(s))
	copy(a, s)

	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}

	return a
}
