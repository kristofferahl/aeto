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

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types" // Required for Watching
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"            // Required for Watching
	kreconcile "sigs.k8s.io/controller-runtime/pkg/reconcile" // Required for Watching
	"sigs.k8s.io/controller-runtime/pkg/source"               // Required for Watching

	acmawsv1alpha1 "github.com/kristofferahl/aeto/apis/acm.aws/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/config"
	"github.com/kristofferahl/aeto/internal/pkg/kubernetes"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

// CertificateConnectorReconciler reconciles a CertificateConnector object
type CertificateConnectorReconciler struct {
	kubernetes.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificateconnectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificateconnectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificateconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups=acm.aws.aeto.net,resources=certificate,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CertificateConnector object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *CertificateConnectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rctx := reconcile.NewContext("certificateconnector", req, log.FromContext(ctx))
	rctx.Log.Info("reconciling")

	var certificateConnector acmawsv1alpha1.CertificateConnector
	if err := r.Get(rctx, req.NamespacedName, &certificateConnector); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	certificates, err := r.getCertificates(rctx, certificateConnector)
	if err != nil {
		return ctrl.Result{}, err
	}

	connector, err := NewConnector(r.Client, certificateConnector)
	if err != nil {
		rctx.Log.Error(err, "failed to create connector", "certificate-connector", certificateConnector.NamespacedName())
		return ctrl.Result{}, nil
	}

	results := reconcile.ResultList{}

	changed, connected, connectRes := connector.Connect(rctx, certificates)
	results = append(results, connectRes)

	statusResult := r.reconcileStatus(rctx, certificateConnector, changed, connected)
	results = append(results, statusResult)

	return rctx.Complete(results...)
}

func (r *CertificateConnectorReconciler) getCertificates(ctx reconcile.Context, connector acmawsv1alpha1.CertificateConnector) ([]acmawsv1alpha1.Certificate, error) {
	selector := connector.Spec.Certificates.Selector

	var list acmawsv1alpha1.CertificateList
	if err := r.List(ctx, &list, selector.ListOptions()); err != nil {
		return []acmawsv1alpha1.Certificate{}, err
	}

	filteredList := make([]acmawsv1alpha1.Certificate, 0)
	for _, item := range list.Items {
		if selector.Match(item.ObjectMeta) {
			filteredList = append(filteredList, item)
		}
	}

	return filteredList, nil
}

func (r *CertificateConnectorReconciler) reconcileStatus(ctx reconcile.Context, connector acmawsv1alpha1.CertificateConnector, changed bool, connected bool) reconcile.Result {
	status := metav1.ConditionFalse
	reason := "CertificatesChanged"
	if !changed {
		if connected {
			status = metav1.ConditionTrue
			reason = "CertificatesConnected"
		} else {
			status = metav1.ConditionFalse
			reason = "CertificatesNotConnected"
		}
	}

	inSyncCondition := metav1.Condition{
		Type:               acmawsv1alpha1.ConditionTypeInSync,
		Status:             status,
		Reason:             reason,
		Message:            "",
		ObservedGeneration: connector.Generation,
	}
	apimeta.SetStatusCondition(&connector.Status.Conditions, inSyncCondition)

	if err := r.UpdateStatus(ctx, &connector); err != nil {
		return ctx.Error(err)
	}
	return ctx.Done()
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertificateConnectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&acmawsv1alpha1.CertificateConnector{}).
		Watches(
			&source.Kind{Type: &acmawsv1alpha1.Certificate{}},
			handler.EnqueueRequestsFromMapFunc(r.findCertificateConnectorsForCertificate),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *CertificateConnectorReconciler) findCertificateConnectorsForCertificate(certificate client.Object) []kreconcile.Request {
	certificateConnectorList := &acmawsv1alpha1.CertificateConnectorList{}
	listOps := &client.ListOptions{
		Namespace: config.Operator.Namespace,
	}
	err := r.Client.GetClient().List(context.TODO(), certificateConnectorList, listOps)
	if err != nil {
		return []kreconcile.Request{}
	}

	requests := make([]kreconcile.Request, len(certificateConnectorList.Items))
	for i, item := range certificateConnectorList.Items {
		requests[i] = kreconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}
