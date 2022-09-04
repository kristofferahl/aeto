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

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"

	acmawsv1alpha1 "github.com/kristofferahl/aeto/apis/acm.aws/v1alpha1"
	awsclients "github.com/kristofferahl/aeto/internal/pkg/aws"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

const (
	FinalizerName = "certificate.acm.aws.aeto.net/finalizer"
)

// CertificateReconciler reconciles a Certificate object
type CertificateReconciler struct {
	kubernetes.Client
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

	var certificate acmawsv1alpha1.Certificate
	if err := r.Get(rctx, req.NamespacedName, &certificate); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := reconcile.NewGenericFinalizer(FinalizerName, func(c reconcile.Context) reconcile.Result {
		certificate := certificate
		return r.reconcileDelete(c, certificate)
	})
	res, err := reconcile.WithFinalizer(r.Client.GetClient(), rctx, &certificate, finalizer)
	if res != nil || err != nil {
		rctx.Log.V(1).Info("returning finalizer results for Certificate", "certificate", certificate.NamespacedName(), "res", res, "error", err)
		return *res, err
	}

	results := reconcile.ResultList{}

	cd, certificateResult := r.reconcileCertificate(rctx, certificate)
	results = append(results, certificateResult)

	if results.AllSuccessful() {
		validationResult := r.reconcileCertificateValidation(rctx, certificate, cd)
		results = append(results, validationResult)
	}

	statusResult := r.reconcileStatus(rctx, certificate, cd, certificateResult.Error())
	results = append(results, statusResult)

	if !apimeta.IsStatusConditionTrue(certificate.Status.Conditions, acmawsv1alpha1.ConditionTypeReady) {
		results = append(results, rctx.RequeueIn(15, "waiting for certificate to be ready"))
	}

	if !apimeta.IsStatusConditionTrue(certificate.Status.Conditions, acmawsv1alpha1.ConditionTypeInUse) {
		results = append(results, rctx.RequeueIn(15, "waiting for certificate to be in use"))
	}

	return rctx.Complete(results...)
}

func (r *CertificateReconciler) reconcileCertificate(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate) (*acmtypes.CertificateDetail, reconcile.Result) {
	ca := ""

	certs, err := r.AWS.FindAcmCertificatesByDomainName(ctx.Context, certificate.Spec.Region, certificate.Spec.DomainName)
	if err != nil {
		ctx.Log.Error(err, "failed to find AWS ACM Certificate summaries matching domain name", "region", r.AWS.Region(certificate.Spec.Region), "domain-name", certificate.Spec.DomainName)
		return nil, ctx.Error(err)
	}

	ownerTags := map[string]string{
		"ManagedBy": "aeto",
		"OwnerRef":  fmt.Sprintf("%s-%s", certificate.Kind, certificate.UID),
	}

	matchedArns := make([]string, 0)
	for _, cs := range certs {
		tags, err := r.AWS.ListAcmCertificateTagsByArn(ctx.Context, certificate.Spec.Region, *cs.CertificateArn)
		if err != nil {
			var rnfe *acmtypes.ResourceNotFoundException
			if errors.As(err, &rnfe) {
				ctx.Log.Info("AWS ACM Certificate tags not found", "region", r.AWS.Region(certificate.Spec.Region), "arn", *cs.CertificateArn)
				return nil, ctx.RequeueIn(5, fmt.Sprintf("AWS ACM Certificate tags not found for arn %s", *cs.CertificateArn))
			} else {
				ctx.Log.Error(err, "failed to fetch AWS ACM Certificate tags", "region", r.AWS.Region(certificate.Spec.Region), "arn", *cs.CertificateArn)
				return nil, ctx.Error(err)
			}
		}

		tagsMatchOwner := true
		for key, val := range ownerTags {
			if v, ok := tags[key]; !ok || v != val {
				tagsMatchOwner = false
				break
			}
		}

		if tagsMatchOwner {
			matchedArns = append(matchedArns, *cs.CertificateArn)
		}
	}

	if len(matchedArns) > 1 {
		err := fmt.Errorf("multiple AWS ACM Certificates match the domain name %s and tags %v, unabled to determine certificate ownership", certificate.Spec.DomainName, ownerTags)
		return nil, ctx.Error(err)
	}

	if len(matchedArns) == 1 {
		ca = matchedArns[0]
	}

	// Merge owner and spec tags
	certificateTags := make(map[string]string)
	for key, value := range certificate.Spec.Tags {
		certificateTags[key] = value
	}
	for key, value := range ownerTags {
		certificateTags[key] = value
	}

	// No arn set, try and create it
	if ca == "" {
		ctx.Log.Info("creating a new AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "domain-name", certificate.Spec.DomainName)
		arn, err := r.newAcmCertificate(ctx, certificate, certificateTags)
		if err != nil {
			ctx.Log.Error(err, "failed to create AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "domain-name", certificate.Spec.DomainName)
			return nil, ctx.Error(err)
		}
		ca = arn
	}

	// Arn set, get certificate and set tags
	if ca != "" {
		cd, err := r.AWS.GetAcmCertificateDetailsByArn(ctx.Context, certificate.Spec.Region, ca)
		if err != nil {
			var rnfe *acmtypes.ResourceNotFoundException
			if errors.As(err, &rnfe) {
				ctx.Log.Info("AWS ACM Certificate details not found", "arn", ca)
				return nil, ctx.RequeueIn(5, fmt.Sprintf("AWS ACM Certificate details not found for arn %s", ca))
			} else {
				ctx.Log.Error(err, "failed to fetch AWS ACM Certificate details", "region", r.AWS.Region(certificate.Spec.Region), "arn", ca)
				return nil, ctx.Error(err)
			}
		}

		ctx.Log.V(1).Info("reconciling tags for AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "domain-name", cd.DomainName, "arn", cd.CertificateArn, "tags", certificate.Spec.Tags)
		err = r.AWS.SetAcmCertificateTagsByArn(ctx.Context, certificate.Spec.Region, ca, certificateTags)
		if err != nil {
			ctx.Log.Error(err, "failed to reconcile tags for AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "domain-name", certificate.Spec.DomainName, "arn", *cd.CertificateArn, "tags", certificate.Spec.Tags)
			return nil, ctx.Error(err)
		}

		return &cd, ctx.Done()
	}

	return nil, ctx.RequeueIn(15, "no AWS ACM Certificate arn set")
}

func (r *CertificateReconciler) reconcileCertificateValidation(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate, details *acmtypes.CertificateDetail) reconcile.Result {
	if details == nil {
		ctx.Log.V(1).Info("waiting for details on AWS ACM Certificate to become available, skipping reconcile of certificate validation")
		return ctx.RequeueIn(15, "waiting for details on AWS ACM Certificate to become available")
	}

	if certificate.Spec.Validation != nil {
		if certificate.Spec.Validation.Dns != nil {
			return r.reconcileCertificateDnsValidationRecord(ctx, certificate.Spec.Validation.Dns.HostedZonedId, *details)
		}

		ctx.Log.Info("unhandled status for AWS ACM Certificate", "status", details.Status, "region", r.AWS.Region(certificate.Spec.Region), "arn", *details.CertificateArn)
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
		ctx.Log.V(1).Info("AWS ACM Certificate domain validation option is missing it's resource record", "domain-name", details.DomainName, "arn", details.CertificateArn)
		return ctx.RequeueIn(5, "AWS ACM Certificate domain validation option is missing it's resource record")
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

	if details.Status == acmtypes.CertificateStatusPendingValidation {
		return ctx.RequeueIn(15, "waiting for domain validation to complete")
	}

	return ctx.Done()
}

func (r *CertificateReconciler) reconcileDelete(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate) reconcile.Result {
	ca := certificate.Status.Arn

	// No arn set, nothing we can do
	if ca == "" {
		return ctx.Done()
	}

	cd, err := r.AWS.GetAcmCertificateDetailsByArn(ctx.Context, certificate.Spec.Region, ca)
	if err != nil {
		var rnfe *acmtypes.ResourceNotFoundException
		if errors.As(err, &rnfe) {
			ctx.Log.Info("AWS ACM Certificate details not found", "region", r.AWS.Region(certificate.Spec.Region), "arn", ca)
			return ctx.Done()
		} else {
			ctx.Log.Error(err, "failed to fetch AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "arn", ca)
			return ctx.Error(err)
		}
	}

	// DNS Validation Records
	if certificate.Spec.Validation != nil && certificate.Spec.Validation.Dns != nil {
		for _, dvo := range cd.DomainValidationOptions {
			if dvo.ValidationMethod == acmtypes.ValidationMethodDns {
				// Delete validation recordset
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

				ctx.Log.Info("deleting Domain Validation record for AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "arn", ca, "hosted-zone-id", certificate.Spec.Validation.Dns.HostedZonedId, "recordset", recordSet)
				err := r.AWS.DeleteRoute53ResourceRecordSet(ctx.Context, certificate.Spec.Validation.Dns.HostedZonedId, recordSet, "deleting DNS validation record for AWS ACM Certificate")
				if err != nil {
					var oe *smithy.OperationError
					if errors.As(err, &oe) {
						if strings.Contains(oe.Error(), "NoSuchHostedZone") {
							ctx.Log.Info("ignoring NoSuchHostedZone error", "message", oe.Error())
							continue
						}
					}

					var cbe *route53types.InvalidChangeBatch
					if errors.As(err, &cbe) {
						if strings.Contains(cbe.ErrorMessage(), "not found") {
							ctx.Log.Info("ignoring InvalidChangeBatch error", "message", cbe.ErrorMessage())
							continue
						}
					}

					return ctx.Error(err)
				}
			}
		}
	}

	// Certificate
	ctx.Log.Info("deleting AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "arn", ca)
	_, err = r.AWS.Acm(certificate.Spec.Region).DeleteCertificate(ctx.Context, &acm.DeleteCertificateInput{
		CertificateArn: aws.String(ca),
	})
	if err != nil {
		var riue *acmtypes.ResourceInUseException
		if errors.As(err, &riue) {
			return ctx.RequeueIn(15, fmt.Sprintf("failed to delete AWS ACM Certificate %s as it is currently in use", ca))
		}
		ctx.Log.Error(err, "failed to delete AWS ACM Certificate", "region", r.AWS.Region(certificate.Spec.Region), "arn", ca)
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *CertificateReconciler) reconcileStatus(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate, cd *acmtypes.CertificateDetail, reconcileErr bool) reconcile.Result {
	ready := metav1.ConditionFalse
	inUse := metav1.ConditionFalse
	if cd == nil {
		certificate.Status.Arn = ""
		certificate.Status.Status = "Unknown"
		if reconcileErr {
			certificate.Status.Status = "Error"
		}
	} else {
		certificate.Status.Arn = *cd.CertificateArn
		certificate.Status.Status = string(cd.Status)
		if *cd.CertificateArn != "" && cd.Status == acmtypes.CertificateStatusIssued {
			ready = metav1.ConditionTrue
		}
		if len(cd.InUseBy) > 0 {
			inUse = metav1.ConditionTrue
		}
	}

	readyCondition := metav1.Condition{
		Type:               acmawsv1alpha1.ConditionTypeReady,
		Status:             ready,
		Reason:             certificate.Status.Status,
		Message:            "",
		ObservedGeneration: certificate.Generation,
	}
	apimeta.SetStatusCondition(&certificate.Status.Conditions, readyCondition)
	inUseCondition := metav1.Condition{
		Type:               acmawsv1alpha1.ConditionTypeInUse,
		Status:             inUse,
		Reason:             certificate.Status.Status,
		Message:            "",
		ObservedGeneration: certificate.Generation,
	}
	apimeta.SetStatusCondition(&certificate.Status.Conditions, inUseCondition)

	if err := r.UpdateStatus(ctx, &certificate); err != nil {
		return ctx.Error(err)
	}

	return ctx.Done()
}

func (r *CertificateReconciler) newAcmCertificate(ctx reconcile.Context, certificate acmawsv1alpha1.Certificate, certificateTags map[string]string) (arn string, err error) {
	req := &acm.RequestCertificateInput{
		DomainName:       aws.String(certificate.Spec.DomainName),
		IdempotencyToken: aws.String(strings.ReplaceAll(certificate.GetNamespace()+"_"+certificate.GetName(), "-", "")),
		ValidationMethod: acmtypes.ValidationMethodDns,
	}

	tags := make([]acmtypes.Tag, 0)
	for key, value := range certificateTags {
		tags = append(tags, acmtypes.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	if len(tags) > 0 {
		req.Tags = tags
	}

	res, err := r.AWS.Acm(certificate.Spec.Region).RequestCertificate(ctx.Context, req)
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
