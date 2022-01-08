package config

import "time"

var (
	Operator OperatorConfig
)

// OperatorConfig contains the configuration options of the operator
type OperatorConfig struct {
	ReconcileInterval time.Duration
	Namespace         string
}
