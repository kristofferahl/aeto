package controllers

import (
	"context"

	"github.com/go-logr/logr"
	guuid "github.com/google/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileContext provides context during reconciles
type ReconcileContext struct {
	Request ctrl.Request
	Context context.Context
	Log     logr.Logger
}

// NewReconcileContext creates a new context
func NewReconcileContext(name string, req ctrl.Request, log logr.Logger) ReconcileContext {
	id := []rune(guuid.New().String())
	cid := string(id[0:7])
	return ReconcileContext{
		Context: context.Background(),
		Log:     log.WithValues(name, req.NamespacedName, "cid", cid),
		Request: req,
	}
}
