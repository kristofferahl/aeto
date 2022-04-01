package acmaws

import (
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	acmawsv1alpha1 "github.com/kristofferahl/aeto/apis/acm.aws/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
	"github.com/kristofferahl/aeto/internal/pkg/util"
)

const (
	// AlbIngressControllerIngressAnnotation_CertificateArnKey is the annotation key used by ALB Ingress Controller to look for certificates to assign to the ALB listener for HTTPS
	AlbIngressControllerIngressAnnotation_CertificateArnKey = "alb.ingress.kubernetes.io/certificate-arn"

	// AlbIngressControllerIngressAnnotation_OperatorOriginalCertificateArnKey is the annotation key containing a static set of certificate arns to include in the final list of arns
	AlbIngressControllerIngressAnnotation_OperatorStaticCertificateArnKey = "acm.aws.aeto.net/static-certificate-arn"

	// AlbIngressControllerIngressAnnotation_OperatorControlled indicates that the ingress resource is controlled by the operator when the value is set to true
	AlbIngressControllerIngressAnnotation_OperatorControlled = "aeto.net/controlled"
)

// AlbIngressControllerConnector defines a connector of certificates for use with ALB Ingress Contoller
type AlbIngressControllerConnector struct {
	kubernetes.Client
	Spec acmawsv1alpha1.IngressSpec
}

// Connect reconciles certificate connections for ALB Ingress Controller
func (c AlbIngressControllerConnector) Connect(ctx reconcile.Context, certificates []acmawsv1alpha1.Certificate) (changed bool, result reconcile.Result) {
	certificateArns := make([]string, 0)
	for _, certificate := range certificates {
		if apimeta.IsStatusConditionTrue(certificate.Status.Conditions, acmawsv1alpha1.ConditionTypeReady) && certificate.Status.Arn != "" {
			certificateArns = append(certificateArns, certificate.Status.Arn)
		}
	}

	ingresses, ingressRes := c.GetIngressList(ctx)
	if ingressRes.Error() {
		return false, ingressRes
	}

	changes := make([]string, 0)
	errors := make([]error, 0)
	for _, ingress := range ingresses {
		operatorControlledAnnotationValue := ingress.Annotations[AlbIngressControllerIngressAnnotation_OperatorControlled]
		operatorStaticCertArnAnnotationValue := ingress.Annotations[AlbIngressControllerIngressAnnotation_OperatorStaticCertificateArnKey]
		certArnAnnotationValue := ingress.Annotations[AlbIngressControllerIngressAnnotation_CertificateArnKey]

		if operatorControlledAnnotationValue != "true" {
			ctx.Log.Info("Ingress not controlled by operator, missing annotation aeto.net/controlled", "namespace", ingress.Namespace, "name", ingress.Name, "annotations", ingress.GetAnnotations())
			continue
		}

		arns := strings.Split(operatorStaticCertArnAnnotationValue, ",")

		for _, certificateArn := range certificateArns {
			if !util.SliceContainsString(arns, certificateArn) {
				arns = append(arns, certificateArn)
			}
		}

		annotationValue := strings.TrimSuffix(strings.TrimPrefix(strings.Join(arns, ","), ","), ",")
		ingress.Annotations[AlbIngressControllerIngressAnnotation_CertificateArnKey] = annotationValue

		changed := certArnAnnotationValue != ingress.Annotations[AlbIngressControllerIngressAnnotation_CertificateArnKey]
		if changed {
			ctx.Log.V(1).Info("updating Ingress with new certificates", "namespace", ingress.Namespace, "name", ingress.Name, "old-arns", certArnAnnotationValue, "new-arns", ingress.Annotations[AlbIngressControllerIngressAnnotation_CertificateArnKey])
			err := c.Update(ctx, &ingress)
			if err != nil {
				ctx.Log.Error(err, "failed to update Ingress", "namespace", ingress.Namespace, "name", ingress.Name)
				errors = append(errors, err)
				continue
			}

			changes = append(changes, types.NamespacedName{
				Namespace: ingress.Namespace,
				Name:      ingress.Name,
			}.String())
		} else {
			ctx.Log.V(1).Info("Ingress certificates in sync", "namespace", ingress.Namespace, "name", ingress.Name, "current-arns", certArnAnnotationValue)
		}
	}

	if len(errors) == 0 {
		return len(changes) > 0, ctx.Done()
	}

	return len(changes) > 0, ctx.Error(fmt.Errorf("one ore more errors occured when connecting certificates to ingresses; %v", errors))
}

func (c AlbIngressControllerConnector) GetIngressList(ctx reconcile.Context) ([]networkingv1.Ingress, reconcile.Result) {
	selector := c.Spec.Selector

	var list networkingv1.IngressList
	if err := c.List(ctx, &list, selector.ListOptions()); err != nil {
		return []networkingv1.Ingress{}, ctx.Error(err)
	}

	filteredList := make([]networkingv1.Ingress, 0)
	for _, item := range list.Items {
		if selector.Match(item.ObjectMeta) {
			filteredList = append(filteredList, item)
		}
	}

	return filteredList, ctx.Done()
}
