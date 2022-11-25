package sustainability

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/reconcile"
)

func (r SavingsPolicyReconciler) getSecretName(name string) string {
	return fmt.Sprintf("savingspolicy-%s", name)
}

func (r SavingsPolicyReconciler) getSecret(rctx reconcile.Context, name string) (*v1.Secret, error) {
	secret := &v1.Secret{}

	err := r.Get(rctx, client.ObjectKey{
		Namespace: rctx.Request.Namespace,
		Name:      name,
	}, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r SavingsPolicyReconciler) upsertSecret(
	rctx reconcile.Context,
	secretName string,
	savingsPolicy sustainabilityv1alpha1.SavingsPolicy,
	secret *v1.Secret,
	newStringData map[string]string,
	newData map[string][]byte,
) error {
	var newSecret = &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: rctx.Request.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: sustainabilityv1alpha1.GroupVersion.String(),
					Kind:       savingsPolicy.Kind,
					Name:       savingsPolicy.Name,
					UID:        savingsPolicy.UID,
				},
			},
		},
		Data:       make(map[string][]byte),
		StringData: make(map[string]string),
	}

	newSecret.StringData = newStringData
	newSecret.Data = newData

	if secret == nil {
		if err := r.Create(rctx, newSecret); err != nil {
			return err
		}
	} else {
		if err := r.Update(rctx, newSecret); err != nil {
			return err
		}
	}

	return nil
}
