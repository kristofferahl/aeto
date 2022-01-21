package reconcile

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	guuid "github.com/google/uuid"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Context provides context during reconciles
type Context struct {
	CorrelationId string
	Request       ctrl.Request
	Context       context.Context
	Log           logr.Logger
}

// NewContext creates a new context
func NewContext(name string, req ctrl.Request, log logr.Logger) Context {
	id := []rune(guuid.New().String())
	cid := string(id[0:7])
	return Context{
		CorrelationId: cid,
		Context:       context.Background(),
		Log:           log.WithValues("cid", cid),
		Request:       req,
	}
}

// Done creates a ReconcileResult that will not trigger a requeue
func (ctx Context) Done() Result {
	return Result{}
}

// RequeueIn creates a ReconcileResult that will trigger a requeue in 'n' seconds
func (ctx Context) RequeueIn(seconds int) Result {
	return Result{
		RequeueIn: time.Duration(seconds) * time.Second,
	}
}

// Error creates a ReconcileResult that will trigger a requeue based on the given error
func (ctx Context) Error(err error) Result {
	return Result{
		err: err,
	}
}

// Complete handles multiple reconcile results and triggers a requeue based on the results
func (ctx Context) Complete(results ...Result) (ctrl.Result, error) {
	for _, result := range results {
		if result.Requeue() {
			return result.RequeueRequest(ctx)
		}
	}

	ctx.Log.Info("finished reconciliation", "requeue-interval", config.Operator.ReconcileInterval)
	return ctrl.Result{RequeueAfter: config.Operator.ReconcileInterval}, nil
}
