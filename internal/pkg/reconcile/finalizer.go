package reconcile

import (
	"fmt"
	"time"

	"github.com/kristofferahl/aeto/internal/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Finalizer interface {
	Name() string
	Handler() func(Context) Result
}

func WithFinalizer(client client.Client, ctx Context, obj client.Object, finalizer Finalizer) (*ctrl.Result, error) {
	objTypeName := obj.GetObjectKind().GroupVersionKind().Kind

	// Examine DeletionTimestamp to determine if object is under deletion
	if obj.GetDeletionTimestamp().IsZero() {
		if !util.SliceContainsString(obj.GetFinalizers(), finalizer.Name()) {
			ctx.Log.V(1).Info(fmt.Sprintf("ensuring finalizer is present on %s", objTypeName), "finalizer", finalizer.Name())
			finalizers := append(obj.GetFinalizers(), finalizer.Name())
			obj.SetFinalizers(finalizers)
			if err := client.Update(ctx.Context, obj); err != nil {
				return &ctrl.Result{}, err
			}
			ctx.Log.V(1).Info(fmt.Sprintf("finalizer set on %s", objTypeName), "finalizer", finalizer.Name())
		}
	} else {
		// The object is being deleted
		if util.SliceContainsString(obj.GetFinalizers(), finalizer.Name()) {
			ctx.Log.Info(fmt.Sprintf("%s is being deleted, finalizer is present", objTypeName), "finalizer", finalizer.Name())

			result := finalizer.Handler()(ctx)
			if result.RequiresRequeue() {
				ctx.Log.V(1).Info(fmt.Sprintf("finalizer requires requeue for %s", objTypeName), "reason", result.requeueReason, "result", result, "finalizer", finalizer.Name())
				res, err := result.asCtrlResultError()
				return &res, err
			}

			ctx.Log.V(1).Info(fmt.Sprintf("removing finalizer for %s", objTypeName), "finalizer", finalizer.Name())
			finalizers := util.SliceRemoveString(obj.GetFinalizers(), finalizer.Name())
			obj.SetFinalizers(finalizers)
			if err := client.Update(ctx.Context, obj); err != nil {
				return &ctrl.Result{}, err
			}
			time.Sleep(5 * time.Second) // NOTE: Sleeping to stop new reconcile where object still exists in cache
			ctx.Log.Info(fmt.Sprintf("finalizer removed for %s", objTypeName), "finalizer", finalizer.Name())
		}

		return &ctrl.Result{}, nil
	}

	return nil, nil
}

func FinalizerInProgress(r *ctrl.Result, err error) bool {
	return r != nil || err != nil
}
