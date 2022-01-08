package config

import "time"

// OperatorConfig contains the configuration options of the operator
type OperatorConfig struct {
	ReconcileInterval time.Duration
}
