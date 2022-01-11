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

// ResultList contains a list of results
type ResultList []Result

// Success returns true when all result represents a successful reconcile attempt
func (rl ResultList) Success() bool {
	for _, rr := range rl {
		if rr.IsError() {
			return false
		}
	}
	return true
}

// IsError returns true when the result represents a failed reconcile attempt
func (rr Result) IsError() bool {
	return rr.Error != nil
}

// Requeue returns true when there was an error or a requeue was requested
func (rr Result) Requeue() bool {
	return rr.IsError() || rr.RequeueIn.String() != "0s"
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
