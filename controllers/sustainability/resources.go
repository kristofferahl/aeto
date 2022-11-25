package sustainability

import (
	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

type Resources struct {
	client      kubernetes.Client
	rctx        reconcile.Context
	deployments Resource
}

type Resource interface {
	HasResource() bool
	Sleep(c kubernetes.Client, rctx reconcile.Context) error
	WakeUp(c kubernetes.Client, rctx reconcile.Context) error
	Info() ([]byte, error)
}

func NewResources(c kubernetes.Client, rctx reconcile.Context, savingspolicy sustainabilityv1alpha1.SavingsPolicy, data SavingsPolicyData) (Resources, error) {
	deployments, err := NewDeploymentResource(c, rctx, savingspolicy, data.DeploymentsInfo)
	if err != nil {
		return Resources{}, err
	}

	return Resources{
		client:      c,
		rctx:        rctx,
		deployments: deployments,
	}, nil
}

func (r Resources) HasResources() bool {
	return r.deployments.HasResource()
}

func (r Resources) Sleep() error {
	if r.deployments.HasResource() {
		return r.deployments.Sleep(r.client, r.rctx)
	}

	return nil
}

func (r Resources) WakeUp() error {
	if r.deployments.HasResource() {
		return r.deployments.WakeUp(r.client, r.rctx)
	}

	return nil
}

func (r Resources) Info() (map[string][]byte, error) {
	data := make(map[string][]byte)

	deploymentInfo, err := r.deployments.Info()
	if err != nil {
		return nil, err
	}
	if deploymentInfo != nil {
		data[secretKeyDeployments] = deploymentInfo
	}

	return data, nil
}
