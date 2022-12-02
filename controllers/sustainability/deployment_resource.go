package sustainability

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

const (
	DeploymentResourceApiVersion string = "apps/v1"
	DeploymentResourceKind       string = "Deployment"
)

type DeploymentResource struct {
	deployments []appsv1.Deployment
	replicas    []DeploymentReplicas
}

type DeploymentReplicas struct {
	Name     string `json:"name"`
	Replicas int32  `json:"replicas"`
}

func NewDeploymentResource(c kubernetes.Client, rctx reconcile.Context, savingspolicy sustainabilityv1alpha1.SavingsPolicy, replicas []DeploymentReplicas) (DeploymentResource, error) {
	r := DeploymentResource{
		replicas: replicas,
	}

	if hasDeploymentTargets(savingspolicy) {
		var deployments appsv1.DeploymentList
		if err := c.List(rctx, &deployments, &client.ListOptions{Namespace: rctx.Request.Namespace}); err != nil {
			return r, err
		}

		filtered := make([]appsv1.Deployment, 0)

		for _, d := range deployments.Items {
			if ignoreDeploymentTarget(savingspolicy, d) {
				replicas, found := r.originalReplicas(d.Name)
				if found && replicas > 0 {
					rctx.Log.V(1).Info("ignoring previously targeted deployment, trying wake up before ignoring", "deployment", d.Name, "previous-replicas", replicas)
					err := r.wakeUpDeployment(c, rctx, d)
					if err != nil {
						rctx.Log.Error(err, "failed to wake up previously targeted deployment, will have to retry before ignoring", "deployment", d.Name, "previous-replicas", replicas)
						return r, err
					}
				}
				rctx.Log.V(1).Info("ignoring deployment", "deployment", d.Name)
			} else {
				filtered = append(filtered, d)
			}
		}

		r.deployments = filtered
	}

	return r, nil
}

func (r DeploymentResource) HasResource() bool {
	return len(r.deployments) > 0
}

func (r DeploymentResource) Sleep(c kubernetes.Client, rctx reconcile.Context) error {
	for _, d := range r.deployments {
		rctx.Log.V(1).Info("ensuring deployment i scaled to 0", "deployment", d.Name)
		if *d.Spec.Replicas != 0 {
			if err := r.scaleTo(c.GetClient(), rctx, d, 0, *d.Spec.Replicas); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r DeploymentResource) WakeUp(c kubernetes.Client, rctx reconcile.Context) error {
	for _, d := range r.deployments {
		err := r.wakeUpDeployment(c, rctx, d)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r DeploymentResource) Info() ([]byte, error) {
	deploymentReplicas := []DeploymentReplicas{}

	for _, d := range r.deployments {
		originalReplicas := *d.Spec.Replicas
		if replica, ok := r.originalReplicas(d.Name); ok && replica != 0 {
			originalReplicas = replica
		}
		if originalReplicas == 0 {
			continue
		}
		deploymentReplicas = append(deploymentReplicas, DeploymentReplicas{
			Name:     d.Name,
			Replicas: originalReplicas,
		})
	}

	return json.Marshal(deploymentReplicas)
}

func (r DeploymentResource) wakeUpDeployment(c kubernetes.Client, rctx reconcile.Context, d appsv1.Deployment) error {
	if *d.Spec.Replicas != 0 {
		rctx.Log.V(1).Info("deployment replicas not set to 0, skipping wake up", "deployment", d.Name)
		return nil
	}

	replicas, ok := r.originalReplicas(d.Name)
	if !ok {
		rctx.Log.Info("deployment not tracked in state, unable to wake up", "deployment", d.Name)
		return nil
	}

	if *d.Spec.Replicas != replicas {
		if err := r.scaleTo(c.GetClient(), rctx, d, replicas, *d.Spec.Replicas); err != nil {
			return err
		}
	}

	return nil
}

func (r DeploymentResource) originalReplicas(name string) (int32, bool) {
	for _, r := range r.replicas {
		if r.Name == name {
			return r.Replicas, true
		}
	}
	return 0, false
}

func (r DeploymentResource) scaleTo(c client.Client, rctx reconcile.Context, deployment appsv1.Deployment, replicas int32, fromReplicas int32) error {
	patchString := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
	patch := []byte(patchString)
	err := c.Patch(rctx.Context, &deployment, client.RawPatch(types.StrategicMergePatchType, patch))
	if err == nil {
		rctx.Log.Info("scaled deployment", "deployment", deployment.Name, "from", fromReplicas, "to", replicas)
	}
	return err
}

func ConvertToDeploymentsInfo(data []byte) ([]DeploymentReplicas, error) {
	if data == nil {
		return []DeploymentReplicas{}, nil
	}

	deploymentReplicas := []DeploymentReplicas{}
	if err := json.Unmarshal(data, &deploymentReplicas); err != nil {
		return nil, err
	}

	return deploymentReplicas, nil
}

func hasDeploymentTargets(savingspolicy sustainabilityv1alpha1.SavingsPolicy) bool {
	for _, target := range savingspolicy.Spec.Targets {
		if target.ApiVersion == DeploymentResourceApiVersion && target.Kind == DeploymentResourceKind {
			return true
		}
	}
	return false
}

func ignoreDeploymentTarget(savingspolicy sustainabilityv1alpha1.SavingsPolicy, deployment appsv1.Deployment) bool {
	for _, t := range savingspolicy.Spec.Targets {
		if t.ApiVersion == DeploymentResourceApiVersion && t.Kind == DeploymentResourceKind && (t.Name == "" || t.Name == deployment.Name) && t.Ignore {
			return true
		}
	}
	return false
}
