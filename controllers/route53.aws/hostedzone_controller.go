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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	route53awsv1alpha1 "github.com/kristofferahl/aeto/apis/route53.aws/v1alpha1"
	awsclients "github.com/kristofferahl/aeto/internal/pkg/aws"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

// HostedZoneReconciler reconciles a HostedZone object
type HostedZoneReconciler struct {
	client.Client
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

	hostedZone, err := r.getHostedZone(rctx, req)
	if err != nil {
		rctx.Log.Info("HostedZone not found", req.NamespacedName)
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	rctx.Log.V(1).Info("found HostedZone", "hostedzone", hostedZone.NamespacedName())

	hz, hzns, hostedZoneResult := r.reconcileHostedZone(rctx, hostedZone)
	chz, cns, connectionResult := r.reconcileHostedZoneConnection(rctx, hostedZone, hzns)
	statusRes := r.reconcileStatus(rctx, hostedZone, hz, chz, cns)

	return rctx.Complete(hostedZoneResult, connectionResult, statusRes)
}

func (r *HostedZoneReconciler) getHostedZone(ctx reconcile.Context, req ctrl.Request) (route53awsv1alpha1.HostedZone, error) {
	var hostedZone route53awsv1alpha1.HostedZone
	if err := r.Get(ctx.Context, req.NamespacedName, &hostedZone); err != nil {
		return route53awsv1alpha1.HostedZone{}, err
	}

	return hostedZone, nil
}

func (r *HostedZoneReconciler) reconcileHostedZone(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone) (*types.HostedZone, *types.ResourceRecordSet, reconcile.Result) {
	hz, err := r.AWS.GetRoute53HostedZoneByName(ctx.Context, hostedZone.Spec.Name)
	if err != nil {
		ctx.Log.Info("AWS Route53 HostedZone not found, creating", "name", hostedZone.Spec.Name)

		hz, err = r.newHostedZone(ctx, hostedZone.Spec.Name)
		if err != nil {
			ctx.Log.Error(err, "failed to create AWS Route53 HostedZone", "name", hostedZone.Spec.Name)
			return nil, nil, ctx.Error(err)
		}
	} else {
		ctx.Log.V(1).Info("AWS Route53 HostedZone found", "name", *hz.Name, "id", *hz.Id)
	}

	if hostedZone.Spec.ConnectWith != nil {
		hzns, err := r.getHostedZoneNsRecordSet(ctx, *hz.Id, *hz.Name)
		if err != nil {
			ctx.Log.Error(err, "NS recordset not found for AWS Route53 HostedZone", "name", *hz.Name, "id", *hz.Id)
			return &hz, nil, ctx.Error(err)
		}

		return &hz, &hzns, ctx.Done()
	}

	return &hz, nil, ctx.Done()
}

func (r *HostedZoneReconciler) reconcileHostedZoneConnection(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone, nsRecordSet *types.ResourceRecordSet) (*types.HostedZone, *types.ResourceRecordSet, reconcile.Result) {
	if hostedZone.Spec.ConnectWith == nil {
		// All good, nothing to do
		return nil, nil, ctx.Done()
	}

	if nsRecordSet == nil {
		return nil, nil, ctx.RequeueIn(5)
	}

	phz, err := r.AWS.GetRoute53HostedZoneByName(ctx.Context, hostedZone.Spec.ConnectWith.Name)
	if err != nil {
		ctx.Log.Error(err, "AWS Route53 HostedZone HostedZone not found", "name", hostedZone.Spec.ConnectWith.Name)
		return nil, nil, ctx.Error(err)
	}

	phzns, err := r.getHostedZoneNsRecordSet(ctx, *phz.Id, *nsRecordSet.Name)
	if err != nil {
		ctx.Log.V(1).Info("NS recordset not found for AWS Route53 HostedZone, creating", "name", phz.Name, "id", phz.Id, "ns", *nsRecordSet)

		err = r.upsertHostedZoneNSRecordSet(ctx.Context, phz, *nsRecordSet, hostedZone.Spec.ConnectWith.TTL)
		if err != nil {
			return &phz, nil, ctx.Error(err)
		}
	}

	return &phz, &phzns, ctx.Done()
}

func (r *HostedZoneReconciler) reconcileStatus(ctx reconcile.Context, hostedZone route53awsv1alpha1.HostedZone, hz *types.HostedZone, phz *types.HostedZone, phzns *types.ResourceRecordSet) reconcile.Result {
	hostedZone.Status.State = "Creating"
	hostedZone.Status.Ready = false

	if hz != nil {
		hostedZone.Status.Id = strings.ReplaceAll(*hz.Id, "/hostedzone/", "")
		hostedZone.Status.State = "Created"
		hostedZone.Status.ConnectedTo = ""
		hostedZone.Status.Ready = true

		if hostedZone.Spec.ConnectWith != nil {
			if phz == nil || phzns == nil {
				hostedZone.Status.Ready = false
			} else {
				hostedZone.Status.ConnectedTo = strings.TrimSuffix(*phz.Name, ".")
				hostedZone.Status.Ready = true
			}
		}
	}

	ctx.Log.V(1).Info("updating HostedZone status")
	if err := r.Status().Update(ctx.Context, &hostedZone); err != nil {
		ctx.Log.Error(err, "failed to update HostedZone status")
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *HostedZoneReconciler) newHostedZone(ctx reconcile.Context, name string) (types.HostedZone, error) {
	id := ctx.CorrelationId
	zone, err := r.AWS.Route53.CreateHostedZone(ctx.Context, &route53.CreateHostedZoneInput{
		Name:            aws.String(name),
		CallerReference: aws.String(id),
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

func (r *HostedZoneReconciler) getHostedZoneNsRecordSet(ctx reconcile.Context, hostedZoneID string, recordName string) (types.ResourceRecordSet, error) {
	res, err := r.AWS.Route53.ListResourceRecordSets(ctx.Context, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
	})
	if err != nil {
		return types.ResourceRecordSet{}, err
	}

	for _, rs := range res.ResourceRecordSets {
		match := rs.Type == types.RRTypeNs && *rs.Name == recordName
		if match {
			return rs, nil
		}
	}

	return types.ResourceRecordSet{}, fmt.Errorf("NS recordset \"%s\" not found (truncated=%v)", recordName, res.IsTruncated)
}

func (r *HostedZoneReconciler) upsertHostedZoneNSRecordSet(ctx context.Context, hostedZone types.HostedZone, nsRecordSet types.ResourceRecordSet, ttl int64) error {
	params := route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name:            nsRecordSet.Name,
						Type:            types.RRTypeNs,
						ResourceRecords: nsRecordSet.ResourceRecords,
						TTL:             aws.Int64(ttl),
					},
				},
			},
			Comment: aws.String("Upserting NS recordset in Hosted Zone"),
		},
		HostedZoneId: hostedZone.Id,
	}

	_, err := r.AWS.Route53.ChangeResourceRecordSets(ctx, &params)
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HostedZoneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&route53awsv1alpha1.HostedZone{}).
		Complete(r)
}
