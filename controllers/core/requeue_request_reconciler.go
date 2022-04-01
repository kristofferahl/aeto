package core

import (
	"time"

	"github.com/kristofferahl/aeto/internal/pkg/eventsource"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/tenant"
)

func ReconcileRequeueRequest(ctx reconcile.Context, stream eventsource.Stream) reconcile.Result {
	rr := reconcile.Result{}
	handler := &RequeueRequestEventHandler{
		state: &rr,
	}
	res := eventsource.Replay(handler, stream.Events())
	if res.Failed() {
		ctx.Log.Error(res.Error, "failed to replay RequeueRequest from events")
		return ctx.Error(res.Error)
	}

	return rr
}

type RequeueRequestEventHandler struct {
	state *reconcile.Result
}

func (h *RequeueRequestEventHandler) On(e eventsource.Event) {
	// TODO: Verify we requeue on failed resource generation
	switch e.(type) {
	case *tenant.ResourceGenererationFailed:
		h.state.RequeueIn(15*time.Second, "resource generation failed")
		break
	case *tenant.ResourceGenererationSuccessful:
		h.state.RequeueIn(0*time.Second, "resource generation failed")
		break
	}
}
