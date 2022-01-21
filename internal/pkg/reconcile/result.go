package reconcile

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// Result gives context about the result of a reconcile operation
type Result struct {
	err       error
	RequeueIn time.Duration
}

// ResultList contains a list of results
type ResultList []Result

// Done returns true when all result represents a successful and completed reconcile attempt
func (rl ResultList) Done() bool {
	for _, rr := range rl {
		if rr.Requeue() {
			return false
		}
	}
	return true
}

// Success returns true when all result represents a successful reconcile attempt
func (rl ResultList) Success() bool {
	for _, rr := range rl {
		if rr.Error() {
			return false
		}
	}
	return true
}

// Error returns true when the result represents a failed reconcile attempt
func (rr Result) Error() bool {
	return rr.err != nil
}

// Requeue returns true when there was an error or a requeue was requested
func (rr Result) Requeue() bool {
	return rr.Error() || rr.RequeueIn.String() != "0s"
}

// RequeueRequest logs and returns a controller runtime result with a request to requeue
func (rr Result) RequeueRequest(ctx Context) (ctrl.Result, error) {
	if rr.err == nil {
		ctx.Log.Info("reconciliation still in progress", "requeue-in", rr.RequeueIn)
	}

	return rr.AsCtrlResultError()
}

// AsCtrlResultError converts the results and returns a controller runtime result and error
func (rr Result) AsCtrlResultError() (ctrl.Result, error) {
	if rr.RequeueIn.String() == "0s" {
		return ctrl.Result{}, rr.err
	}

	return ctrl.Result{RequeueAfter: rr.RequeueIn}, rr.err
}
