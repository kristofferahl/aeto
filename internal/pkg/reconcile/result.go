package reconcile

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// Result gives context about the result of a reconcile operation
type Result struct {
	Error     error
	RequeueIn time.Duration
}

// Requeue returns true when error was found or a requeue was requested
func (rr Result) Requeue() bool {
	return rr.Error != nil || rr.RequeueIn.String() != "0s"
}

// RequeueRequest logs and returns a controller runtime result with a request to requeue
func (rr Result) RequeueRequest(ctx Context) (ctrl.Result, error) {
	if rr.Error == nil {
		ctx.Log.Info("reconciliation in progress", "requeue-interval", rr.RequeueIn)
	}

	if rr.RequeueIn.String() == "0s" {
		return ctrl.Result{}, rr.Error
	}
	return ctrl.Result{RequeueAfter: rr.RequeueIn}, rr.Error
}
