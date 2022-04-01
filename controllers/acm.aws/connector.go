package acmaws

import (
	"fmt"

	acmawsv1alpha1 "github.com/kristofferahl/aeto/apis/acm.aws/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

// Connector defines a connector of certificates
type Connector interface {
	// Connect reconciles certificate connections
	Connect(ctx reconcile.Context, certificates []acmawsv1alpha1.Certificate) (changed bool, result reconcile.Result)
}

// NewConnector returns a Connector
func NewConnector(client kubernetes.Client, cc acmawsv1alpha1.CertificateConnector) (Connector, error) {
	if cc.Spec.Ingress != nil {
		switch cc.Spec.Ingress.Connector {
		case acmawsv1alpha1.ConnectorTypeAlbIngressController:
			return AlbIngressControllerConnector{
				Client: client,
				Spec:   *cc.Spec.Ingress,
			}, nil
		default:
			return nil, fmt.Errorf("unhandled ingress connector type %s", cc.Spec.Ingress.Connector)
		}
	}

	return nil, fmt.Errorf("unhandled error, CertificateConnector spec is invalid")
}
