package reconcile

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const requeueIntervalUnset = "0s"

// Result gives context about the result of a reconcile operation
type Result struct {
	err           error
	requeueAfter  time.Duration
	requeueReason string
}

func (rr *Result) RequeueIn(duration time.Duration, reason string) {
	rr.requeueAfter = duration
	rr.requeueReason = reason
}

func (rr *Result) Reset() {
	rr.requeueAfter = 0
	rr.requeueReason = ""
}

// Error returns true when the result represents a failed reconcile attempt
func (rr Result) Error() bool {
	return rr.err != nil
}

// RequiresRequeue returns true when there was an error or a requeue was requested
func (rr Result) RequiresRequeue() bool {
	return rr.Error() || rr.requeueAfter.String() != requeueIntervalUnset
}

func (rr Result) asCtrlResultError() (ctrl.Result, error) {
	if rr.requeueAfter.String() == requeueIntervalUnset {
		return ctrl.Result{}, rr.err
	}

	return ctrl.Result{RequeueAfter: rr.requeueAfter}, rr.err
}
