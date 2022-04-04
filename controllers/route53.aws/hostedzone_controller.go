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

package route53aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	route53awsv1alpha1 "github.com/kristofferahl/aeto/apis/route53.aws/v1alpha1"
	awsclients "github.com/kristofferahl/aeto/internal/pkg/aws"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

const (
	FinalizerName = "hostedzone.route53.aws.aeto.net/finalizer"
)

// HostedZoneReconciler reconciles a HostedZone object
type HostedZoneReconciler struct {
	kubernetes.Client
	Scheme *runtime.Scheme
	AWS    awsclients.Clients
}

//+kubebuilder:rbac:groups=route53.aws.aeto.net,resources=hostedzones,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=route53.aws.aeto.net,resources=hostedzones/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=route53.aws.aeto.net,resources=hostedzones/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HostedZone object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *HostedZoneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("hostedzone", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	var hostedZone route53awsv1alpha1.HostedZone
	if err := r.Get(rctx, req.NamespacedName, &hostedZone); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := reconcile.NewGenericFinalizer(FinalizerName, func(c reconcile.Context) reconcile.Result {
		hostedZone := hostedZone
		return r.reconcileDelete(c, hostedZone)
	})
	res, err := reconcile.WithFinalizer(r.Client.GetClient(), rctx, &hostedZone, finalizer)
	if res != nil || err != nil {
		rctx.Log.V(1).Info("returning finalizer results for HostedZone", "hostedzone", hostedZone.NamespacedName(), "res", res, "error", err)
		return *res, err
	}

	results := reconcile.ResultList{}

	hz, hzns, hostedZoneResult := r.reconcileHostedZone(rctx, hostedZone)
	results = append(results, hostedZoneResult)

	if results.AllSuccessful() {
		chz, cns, connectionResult := r.reconcileHostedZoneConnection(rctx, hostedZone, hzns)
		results = append(results, connectionResult)

		if results.AllSuccessful() {
			// TODO: Always update the status
			statusRes := r.reconcileStatus(rctx, hostedZone, hz, chz, cns)
			results = append(results, statusRes)
		}
	}

	return rctx.Complete(results...)
}

func (r *HostedZoneReconciler) reconcileHostedZone(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone) (*types.HostedZone, *types.ResourceRecordSet, reconcile.Result) {
	id := hostedZone.Status.Id

	// No id set, try and create it
	if id == "" {
		now := time.Now().UTC()
		callerReference := fmt.Sprintf("%s/%s/%s/%s", ctx.Request.String(), hostedZone.Spec.Name, hostedZone.CreationTimestamp.UTC().Format(time.RFC3339), fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute()))
		if len(callerReference) > 128 {
			callerReference = callerReference[:128]
		}
		ctx.Log.Info("creating a new AWS Route53 HostedZone", "name", hostedZone.Spec.Name, "caller-reference", callerReference)
		hz, err := r.newHostedZone(ctx, hostedZone.Spec.Name, callerReference)
		if err != nil {
			ctx.Log.Error(err, "failed to create AWS Route53 HostedZone", "name", hostedZone.Spec.Name, "caller-reference", callerReference)
			return nil, nil, ctx.Error(err)
		}
		id = *hz.Id
	}

	// Id set, get hosted zone and set tags
	if id != "" {
		hz, err := r.AWS.GetRoute53HostedZoneById(ctx.Context, id)
		if err != nil {
			var nshz *types.NoSuchHostedZone
			if errors.As(err, &nshz) {
				ctx.Log.Info("AWS Route53 HostedZone not found", "id", id)
				return nil, nil, ctx.RequeueIn(5, fmt.Sprintf("AWS Route53 HostedZone with id %s was not found", id))
			}

			ctx.Log.Error(err, "failed to fetch AWS Route53 HostedZone", "id", id)
			return nil, nil, ctx.Error(err)
		}

		ctx.Log.V(1).Info("reconciling tags for AWS Route53 HostedZone", "name", *hz.Name, "id", *hz.Id)
		err = r.AWS.SetRoute53HostedZoneTagsById(ctx.Context, *hz.Id, hostedZone.Spec.Tags)
		if err != nil {
			ctx.Log.Error(err, "failed to reconcile tags for AWS Route53 HostedZone", "name", *hz.Name, "id", *hz.Id, "tags", hostedZone.Spec.Tags)
			return &hz, nil, ctx.Error(err)
		}

		if hostedZone.Spec.ConnectWith != nil {
			hzns, err := r.getHostedZoneNsRecordSet(ctx, *hz.Id, *hz.Name)
			if err != nil {
				ctx.Log.Error(err, "failed to fetch NS recordset for AWS Route53 HostedZone", "name", *hz.Name, "id", *hz.Id)
				return &hz, nil, ctx.Error(err)
			}

			return &hz, hzns, ctx.Done()
		}

		return &hz, nil, ctx.Done()
	}

	return nil, nil, ctx.RequeueIn(15, "no AWS Route53 HostedZone id set")
}

func (r *HostedZoneReconciler) reconcileHostedZoneConnection(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone, nsRecordSet *types.ResourceRecordSet) (*types.HostedZone, *types.ResourceRecordSet, reconcile.Result) {
	if hostedZone.Spec.ConnectWith == nil {
		// All good, nothing to do
		return nil, nil, ctx.Done()
	}

	if nsRecordSet == nil {
		return nil, nil, ctx.RequeueIn(5, "NS recordset not yet available for AWS Route53 HostedZone")
	}

	phz, err := r.AWS.FindOneRoute53HostedZoneByName(ctx.Context, hostedZone.Spec.ConnectWith.Name)
	if err != nil {
		ctx.Log.Error(err, "failed to fetch AWS Route53 HostedZone", "name", hostedZone.Spec.ConnectWith.Name)
		return nil, nil, ctx.Error(err)
	}

	if phz == nil {
		ctx.Log.V(1).Info("no matching AWS Route53 HostedZone found, unable to reconcile NS record connection", "name", hostedZone.Spec.ConnectWith.Name)
		return nil, nil, ctx.RequeueIn(15, fmt.Sprintf("no matching AWS Route53 HostedZone found for %s", hostedZone.Spec.ConnectWith.Name))
	}

	nsRecordSet.TTL = &hostedZone.Spec.ConnectWith.TTL
	ctx.Log.V(1).Info("reconciling NS record connection for AWS Route53 HostedZone", "name", *phz.Name, "id", *phz.Id)
	err = r.AWS.UpsertRoute53ResourceRecordSet(ctx.Context, *phz.Id, *nsRecordSet, "Upserting NS recordset in Hosted Zone")
	if err != nil {
		ctx.Log.Error(err, "failed to reconcile NS record connection for AWS Route53 HostedZone", "name", *phz.Name, "id", *phz.Id, "ns", *nsRecordSet)
		return phz, nil, ctx.Error(err)
	}

	phzns, err := r.getHostedZoneNsRecordSet(ctx, *phz.Id, *nsRecordSet.Name)
	if err != nil {
		ctx.Log.V(1).Info("failed to fetch NS recordset for AWS Route53 HostedZone", "name", *phz.Name, "id", *phz.Id)
		return phz, nil, ctx.RequeueIn(15, fmt.Sprintf("failed to fetch NS recordset for AWS Route53 HostedZone %s", *phz.Name))
	}

	return phz, phzns, ctx.Done()
}

func (r *HostedZoneReconciler) reconcileDelete(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone) reconcile.Result {
	id := hostedZone.Status.Id

	// No id set, nothing we can do
	if id == "" {
		return ctx.Done()
	}

	hz, err := r.AWS.GetRoute53HostedZoneById(ctx.Context, id)
	if err != nil {
		var nshz *types.NoSuchHostedZone
		if errors.As(err, &nshz) {
			ctx.Log.Info("AWS Route53 HostedZone not found", "id", id)
			return ctx.Done()
		} else {
			ctx.Log.Error(err, "failed to fetch AWS Route53 HostedZone", "id", id)
			return ctx.Error(err)
		}
	}

	// HostedZone NS recordset in connected zone
	if hostedZone.Spec.ConnectWith != nil {
		phz, err := r.AWS.FindOneRoute53HostedZoneByName(ctx.Context, hostedZone.Spec.ConnectWith.Name)
		if err != nil {
			ctx.Log.Error(err, "failed to fetch AWS Route53 HostedZone", "name", hostedZone.Spec.ConnectWith.Name)
			return ctx.Error(err)
		}
		if phz == nil {
			ctx.Log.V(1).Info("no matching AWS Route53 HostedZone found, unable to delete NS record connection, skipping", "name", hostedZone.Spec.ConnectWith.Name)
		} else {
			phzns, err := r.getHostedZoneNsRecordSet(ctx, *phz.Id, *hz.Name)
			if err != nil {
				ctx.Log.Error(err, "failed to fetch NS recordset for AWS Route53 HostedZone", "name", *phz.Name, "id", *phz.Id, "record-name", *hz.Name)
				return ctx.Error(err)
			}
			if phzns == nil {
				ctx.Log.V(1).Info("no matching NS recordset found, unable to delete NS record connection, skipping", "name", *phz.Name, "id", *phz.Id, "record-name", *hz.Name)
			} else {
				// Delete NS recordset
				ctx.Log.Info("deleting NS record for AWS Route53 HostedZone", "id", *phz.Id, "recordset", *phzns)
				err = r.AWS.DeleteRoute53ResourceRecordSet(ctx.Context, *phz.Id, *phzns, "deleting NS record for AWS Route53 HostedZone")
				if err != nil {
					var cbe *types.InvalidChangeBatch
					if errors.As(err, &cbe) {
						if strings.Contains(cbe.ErrorMessage(), "not found") {
							ctx.Log.Info("ignoring change batch error", "error-message", cbe.ErrorMessage())
						} else {
							return ctx.Error(err)
						}
					} else {
						return ctx.Error(err)
					}
				}
			}
		}
	}

	// HostedZone
	ctx.Log.Info("deleting AWS Route53 HostedZone", "id", *hz.Id, "deletion-policy", hostedZone.Spec.DeletionPolicy)
	err = r.AWS.DeleteRoute53HostedZone(ctx.Context, hz, hostedZone.Spec.DeletionPolicy == route53awsv1alpha1.HostedZoneDeletionPolicyForce)
	if err != nil {
		var hzne *types.HostedZoneNotEmpty
		if errors.As(err, &hzne) {
			ctx.Log.Info("AWS Route53 HostedZone contains non-required resource record sets and cannot be deleted", "id", id)
			return ctx.RequeueIn(60, "AWS Route53 HostedZone contains non-required resource record sets and cannot be deleted") // TODO: Should we requeue with error instead to to utilize backoff strategy?
		} else {
			ctx.Log.Error(err, "failed to delete AWS Route53 HostedZone", "id", id)
			return ctx.Error(err)
		}
	}

	return ctx.Done()
}

func (r *HostedZoneReconciler) reconcileStatus(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone, hz *types.HostedZone, phz *types.HostedZone, phzns *types.ResourceRecordSet) reconcile.Result {
	ready := metav1.ConditionFalse
	if hz == nil {
		hostedZone.Status.Id = ""
		hostedZone.Status.Status = "Creating"
		hostedZone.Status.ConnectedTo = ""
	} else {
		hostedZone.Status.Id = strings.ReplaceAll(*hz.Id, "/hostedzone/", "")
		hostedZone.Status.Status = "Created"
		hostedZone.Status.ConnectedTo = ""
		ready = metav1.ConditionTrue

		if hostedZone.Spec.ConnectWith != nil {
			if phz == nil || phzns == nil {
				ready = metav1.ConditionFalse
			} else {
				hostedZone.Status.ConnectedTo = strings.TrimSuffix(*phz.Name, ".")
				ready = metav1.ConditionTrue
			}
		}
	}

	readyCondition := metav1.Condition{
		Type:    route53awsv1alpha1.ConditionTypeReady,
		Status:  ready,
		Reason:  hostedZone.Status.Status,
		Message: "",
	}
	apimeta.SetStatusCondition(&hostedZone.Status.Conditions, readyCondition)

	if err := r.UpdateStatus(ctx, &hostedZone); err != nil {
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *HostedZoneReconciler) newHostedZone(ctx reconcile.Context, name string, callerReference string) (types.HostedZone, error) {
	zone, err := r.AWS.Route53.CreateHostedZone(ctx.Context, &route53.CreateHostedZoneInput{
		Name:            aws.String(name),
		CallerReference: aws.String(callerReference),
		HostedZoneConfig: &types.HostedZoneConfig{
			Comment:     aws.String("Managed by aeto"),
			PrivateZone: false,
		},
	})
	if err != nil {
		return types.HostedZone{}, err
	}

	return *zone.HostedZone, nil
}

func (r *HostedZoneReconciler) getHostedZoneNsRecordSet(ctx reconcile.Context, hostedZoneID string, recordName string) (*types.ResourceRecordSet, error) {
	res, err := r.AWS.ListRoute53ResourceRecordSets(ctx.Context, hostedZoneID)
	if err != nil {
		return nil, err
	}

	for _, rs := range res {
		match := rs.Type == types.RRTypeNs && *rs.Name == recordName
		if match {
			return &rs, nil
		}
	}

	ctx.Log.V(1).Info("NS recordset not found", "name", recordName)
	return nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HostedZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&route53awsv1alpha1.HostedZone{}).
		Complete(r)
}
