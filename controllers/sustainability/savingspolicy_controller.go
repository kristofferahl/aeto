/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sustainability

import (
	"context"
	"fmt"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

// SavingsPolicyReconciler reconciles a SavingsPolicy object
type SavingsPolicyReconciler struct {
	kubernetes.Client
	Scheme *runtime.Scheme
}

const (
	SavingsPolicyFinalizerName              = "savingspolicy.sustainability.aeto.net/finalizer"
	AnnotationChangedGracePeriodSeconds int = 15
)

//+kubebuilder:rbac:groups=sustainability.aeto.net,resources=savingspolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sustainability.aeto.net,resources=savingspolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sustainability.aeto.net,resources=savingspolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SavingsPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *SavingsPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("savingspolicy", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	var savingspolicy sustainabilityv1alpha1.SavingsPolicy
	if err := r.Get(rctx, req.NamespacedName, &savingspolicy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := reconcile.NewGenericFinalizer(SavingsPolicyFinalizerName, func(c reconcile.Context) reconcile.Result {
		savingspolicy := savingspolicy
		if savingspolicy.Status.Status != sustainabilityv1alpha1.SavingsPolicyTerminating {
			res := r.reconcileStatus(c, savingspolicy, SavingsPolicyData{}, true, false)
			if res.Error() {
				return res
			}
			return c.RequeueIn(5, "status updated, terminating")
		}

		result, _, err := r.reconcileWakeupOrSleep(rctx, true, "terminating SavingsPolicy", savingspolicy)
		if err != nil {
			return rctx.Error(err)
		}
		return result
	})
	res, err := reconcile.WithFinalizer(r.GetClient(), rctx, &savingspolicy, finalizer)
	if reconcile.FinalizerInProgress(res, err) {
		return *res, err
	}

	results := reconcile.ResultList{}

	if results.AllDone() {
		changed, err := r.reconcileSuspendFor(rctx, savingspolicy)
		if err != nil {
			results = append(results, rctx.Error(err))
		} else if changed {
			results = append(results, rctx.RequeueIn(AnnotationChangedGracePeriodSeconds, "SavingsPolicy was updated due to annotation change"))
		}
	}

	if results.AllDone() {
		changed, err := r.reconcileSuspendUntil(rctx, savingspolicy)
		if err != nil {
			results = append(results, rctx.Error(err))
		}
		if changed {
			results = append(results, rctx.RequeueIn(AnnotationChangedGracePeriodSeconds, "SavingsPolicy was updated due to annotation change"))
		}
	}

	if results.AllDone() {
		suspended, reason := checkSchedule(rctx, savingspolicy)
		rctx.Log.Info(reason)

		result, savingsPolicyData, err := r.reconcileWakeupOrSleep(rctx, suspended, reason, savingspolicy)
		if err != nil {
			results = append(results, rctx.Error(err))
		} else {
			results = append(results, result)
		}

		result = r.reconcileStatus(rctx, savingspolicy, savingsPolicyData, false, err != nil)
		results = append(results, result)

		// TODO: Make interval configurable
		results = append(results, rctx.RequeueIn(60, "SavingsPolicy requires continuous reconciliation"))
	}

	return rctx.Complete(results...)
}

func (r *SavingsPolicyReconciler) reconcileWakeupOrSleep(rctx reconcile.Context, suspended bool, reason string, savingspolicy sustainabilityv1alpha1.SavingsPolicy) (reconcile.Result, SavingsPolicyData, error) {
	secretName := r.getSecretName(rctx.Request.Name)
	secret, err := r.getSecret(rctx, secretName)
	if client.IgnoreNotFound(err) != nil {
		rctx.Log.Error(err, "failed to get the SavingsPolicy secret")
		return reconcile.Result{}, SavingsPolicyData{}, err
	}

	savingsPolicyData, err := NewSavingsPolicyData(secret, savingspolicy, suspended, reason)
	if err != nil {
		rctx.Log.Error(err, "failed to create SavingsPolicyData from secret")
		return reconcile.Result{}, SavingsPolicyData{}, err
	}

	resources, err := NewResources(r.Client, rctx, savingspolicy, savingsPolicyData)
	if err != nil {
		return reconcile.Result{}, savingsPolicyData, err
	}

	ssd, sd, err := savingsPolicyData.NewSecretData(resources)
	if err != nil {
		rctx.Log.Error(err, "failed to convert savings policy data to secret data")
	}

	err = r.upsertSecret(rctx, secretName, savingspolicy, secret, ssd, sd)
	if err != nil {
		rctx.Log.Error(err, "failed to upsert the SavingsPolicy secret")
		return reconcile.Result{}, savingsPolicyData, err
	}

	if suspended {
		if err := resources.WakeUp(); err != nil {
			rctx.Log.Error(err, "failed to bring one or more resources out of a sleeping state")
			return rctx.Error(err), savingsPolicyData, nil
		}
	} else {
		if err := resources.Sleep(); err != nil {
			rctx.Log.Error(err, "failed to put one or more resources in a sleeping state")
			return rctx.Error(err), savingsPolicyData, nil
		}
	}

	return rctx.Done(), savingsPolicyData, nil
}

func (r *SavingsPolicyReconciler) reconcileStatus(ctx reconcile.Context, savingspolicy sustainabilityv1alpha1.SavingsPolicy, savingsPolicyData SavingsPolicyData, terminating bool, reconcileErr bool) reconcile.Result {
	savingspolicy.Status.Status = fmt.Sprintf("Targeting %d deployment(s)", len(savingsPolicyData.DeploymentsInfo))
	if terminating {
		savingspolicy.Status.Status = sustainabilityv1alpha1.SavingsPolicyTerminating
	}
	if reconcileErr {
		savingspolicy.Status.Status = sustainabilityv1alpha1.SavingsPolicyError
	}

	if !terminating {
		suspended := metav1.ConditionFalse
		if savingsPolicyData.Suspended {
			suspended = metav1.ConditionTrue
		}

		suspendedCondition := metav1.Condition{
			Type:               sustainabilityv1alpha1.ConditionTypeSuspended,
			Status:             suspended,
			Reason:             "SavingsPolicyStateEvaluated",
			Message:            savingsPolicyData.Reason,
			ObservedGeneration: savingspolicy.Generation,
		}
		apimeta.SetStatusCondition(&savingspolicy.Status.Conditions, suspendedCondition)

		cd := time.Now().UTC().Sub(savingspolicy.ObjectMeta.CreationTimestamp.Time)

		sd := savingsPolicyData.SuspendedDuration
		if savingsPolicyData.Suspended {
			sd = (sd + savingsPolicyData.TimeSinceLastTransition())
			savingspolicy.Status.SuspendedDuration = sd.Round(1 * time.Minute).String()
		}

		ad := (cd - sd)
		savingspolicy.Status.ActiveDuration = ad.Round(1 * time.Minute).String()

		p := (ad.Seconds() / cd.Seconds()) * 100
		savingspolicy.Status.Savings = fmt.Sprintf("%.0f%%", p)
	}

	if err := r.UpdateStatus(ctx, &savingspolicy); err != nil {
		return ctx.Error(err)
	}

	return ctx.Done()
}

// SetupWithManager sets up the controller with the Manager.
func (r *SavingsPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sustainabilityv1alpha1.SavingsPolicy{}).
		Complete(r)
}
