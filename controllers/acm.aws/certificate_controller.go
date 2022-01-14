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

package acmaws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	acmawsv1alpha1 "github.com/kristofferahl/aeto/apis/acm.aws/v1alpha1"
	awsclients "github.com/kristofferahl/aeto/internal/pkg/aws"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

// CertificateReconciler reconciles a Certificate object
type CertificateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	AWS    awsclients.Clients
}

//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Certificate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *CertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("certificate", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	certificate, err := r.getCertificate(rctx, req)
	if err != nil {
		rctx.Log.Info("Certificate not found")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	rctx.Log.V(1).Info("found Certificate", "certificate", certificate.NamespacedName())

	results := reconcile.ResultList{}

	cd, certificateResult := r.reconcileCertificate(rctx, certificate)
	results = append(results, certificateResult)

	if results.Success() {
		validationResult := r.reconcileCertificateValidation(rctx, certificate, cd)
		results = append(results, validationResult)

		statusResult := r.reconcileStatus(rctx, certificate, cd)
		results = append(results, statusResult)
	}

	return rctx.Complete(results...)
}

func (r *CertificateReconciler) getCertificate(ctx reconcile.Context, req ctrl.Request) (acmawsv1alpha1.Certificate, error) {
	var cert acmawsv1alpha1.Certificate
	if err := r.Get(ctx.Context, req.NamespacedName, &cert); err != nil {
		return acmawsv1alpha1.Certificate{}, err
	}
	return cert, nil
}

func (r *CertificateReconciler) reconcileCertificate(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate) (*acmtypes.CertificateDetail, reconcile.Result) {
	ca := certificate.Status.Arn

	// No arn set, try and create it
	if ca == "" {
		ctx.Log.Info("creating a new AWS ACM Certificate", "domain-name", certificate.Spec.DomainName)
		arn, err := r.newAcmCertificate(ctx, certificate)
		if err != nil {
			ctx.Log.Error(err, "failed to create AWS ACM Certificate", "domain-name", certificate.Spec.DomainName)
			return nil, ctx.Error(err)
		}
		ca = arn
	}

	// Arn set, get certificate and set tags
	if ca != "" {
		cd, err := r.AWS.GetAcmCertificateDetailsByArn(ctx.Context, ca)
		if err != nil {
			var rnfe *acmtypes.ResourceNotFoundException
			if errors.As(err, &rnfe) {
				ctx.Log.Info("AWS ACM Certificate details not found", "arn", ca)
				return nil, ctx.RequeueIn(5)
			} else {
				ctx.Log.Error(err, "failed to fetch AWS ACM Certificate", "arn", ca)
				return nil, ctx.Error(err)
			}
		}

		ctx.Log.V(1).Info("reconciling tags for AWS ACM Certificate", "domain-name", cd.DomainName, "arn", cd.CertificateArn, "tags", certificate.Spec.Tags)
		err = r.AWS.SetAcmCertificateTagsByArn(ctx.Context, ca, certificate.Spec.Tags)
		if err != nil {
			ctx.Log.Error(err, "failed to reconcile tags for AWS ACM Certificate", "domain-name", certificate.Spec.DomainName, "arn", *cd.CertificateArn, "tags", certificate.Spec.Tags)
			return nil, ctx.Error(err)
		}

		return &cd, ctx.Done()
	}

	// No arn set yet, retry in 15
	return nil, ctx.RequeueIn(15)
}

func (r *CertificateReconciler) reconcileCertificateValidation(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate, details *acmtypes.CertificateDetail) reconcile.Result {
	if details == nil {
		return ctx.RequeueIn(15)
	}

	if certificate.Spec.Validation != nil {
		switch details.Status {
		case acmtypes.CertificateStatusPendingValidation:
			if certificate.Spec.Validation.Dns != nil {
				return r.reconcileCertificateDnsValidationRecord(ctx, certificate.Spec.Validation.Dns.HostedZonedId, *details)
			}
		case acmtypes.CertificateStatusIssued:
			// TODO: Delete validation record when issued or is it needed for renewal?
			// https://docs.aws.amazon.com/acm/latest/userguide/dns-renewal-validation.html
			break
		default:
			ctx.Log.Info("unhandled status for AWS ACM Certificate", "status", details.Status, "arn", *details.CertificateArn)
		}
	}

	return ctx.Done()
}

func (r *CertificateReconciler) reconcileCertificateDnsValidationRecord(ctx reconcile.Context, hostedZoneId string, details acmtypes.CertificateDetail) reconcile.Result {
	dvoCount := len(details.DomainValidationOptions)
	if dvoCount != 1 {
		return ctx.Error(fmt.Errorf("AWS ACM Certificate domain validation options had %d item(s), expected 1", dvoCount))
	}

	dvo := details.DomainValidationOptions[0]
	if dvo.ValidationMethod != acmtypes.ValidationMethodDns {
		return ctx.Error(fmt.Errorf("AWS ACM Certificate domain validation method mismatch, expected %s but was %s", acmtypes.ValidationMethodDns, dvo.ValidationMethod))
	}

	if dvo.ResourceRecord == nil {
		ctx.Log.V(1).Info("AWS ACM Certificate domain validation option is missing it's resource record, retrying...", "domain-name", details.DomainName, "arn", details.CertificateArn)
		return ctx.RequeueIn(5)
	}

	recordSet := route53types.ResourceRecordSet{
		Name: dvo.ResourceRecord.Name,
		Type: route53types.RRTypeCname,
		ResourceRecords: []route53types.ResourceRecord{
			{
				Value: dvo.ResourceRecord.Value,
			},
		},
		TTL: aws.Int64(300),
	}

	ctx.Log.V(1).Info("reconciling DNS validation record for AWS ACM Certificate", "domain-name", details.DomainName, "arn", details.CertificateArn)
	err := r.AWS.UpsertRoute53ResourceRecordSet(ctx.Context, hostedZoneId, recordSet, "Upserting AWS ACM Certificate domain validation CNAME record in Hosted Zone")
	if err != nil {
		ctx.Log.Error(err, "failed to reconcile AWS ACM Certificate DNS validation record", "domain-name", details.DomainName, "arn", details.CertificateArn)
		return ctx.Error(err)
	}

	// While wating for validation to complete, requeue in short intervals
	return ctx.RequeueIn(15)
}

func (r *CertificateReconciler) reconcileStatus(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate, cd *acmtypes.CertificateDetail) reconcile.Result {
	if cd == nil {
		certificate.Status.Arn = ""
		certificate.Status.State = ""
		certificate.Status.InUse = false
		certificate.Status.Ready = false
	} else {
		certificate.Status.Arn = *cd.CertificateArn
		certificate.Status.State = string(cd.Status)
		certificate.Status.InUse = len(cd.InUseBy) > 0
		certificate.Status.Ready = *cd.CertificateArn != "" && cd.Status == acmtypes.CertificateStatusIssued
	}

	ctx.Log.V(1).Info("updating Certificate status")
	if err := r.Status().Update(ctx.Context, &certificate); err != nil {
		ctx.Log.Error(err, "failed to update Certificate status")
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *CertificateReconciler) newAcmCertificate(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate) (arn string, err error) {
	req := &acm.RequestCertificateInput{
		DomainName:       aws.String(certificate.Spec.DomainName),
		IdempotencyToken: aws.String(strings.ReplaceAll(certificate.GetName(), "-", "")),
		ValidationMethod: acmtypes.ValidationMethodDns,
	}

	tags := make([]acmtypes.Tag, 0)
	for key, value := range certificate.Spec.Tags {
		tags = append(tags, acmtypes.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	if len(tags) > 0 {
		req.Tags = tags
	}

	res, err := r.AWS.Acm.RequestCertificate(ctx.Context, req)
	if err != nil {
		return "", err
	}

	return *res.CertificateArn, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&acmawsv1alpha1.Certificate{}).
		Complete(r)
}
