package testing

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/kristofferahl/aeto/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewTenant(namespace string, name string) (*corev1alpha1.Tenant, types.NamespacedName) {
	spec := corev1alpha1.TenantSpec{
		Name: "Tenant name",
	}

	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	return &corev1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: spec,
	}, key
}

func NewNamespaceTemplate(namespace string, name string) (*corev1alpha1.ResourceTemplate, types.NamespacedName) {
	spec := corev1alpha1.ResourceTemplateSpec{
		Rules: corev1alpha1.ResourceTemplateRules{
			Name:      corev1alpha1.ResourceNameTenant,
			Namespace: corev1alpha1.ResourceNamespaceTenant,
		},
		Resources: []corev1alpha1.EmbeddedResource{
			{
				RawExtension: runtime.RawExtension{
					Raw: []byte(MustConvert(&corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "injected",
							Labels: map[string]string{
								"app.net/tenant": "injected",
							},
							Annotations: map[string]string{
								"app.net/controlled": "injected",
							},
						},
					})),
				},
			},
		},
		Raw: []string{
			`
				apiVersion: v1
				kind: Namespace
				metadata:
				name: "injected"
				labels:
					app.net/tenant: "injected"
				annotations:
					aeto.net/controlled: "injected"
			`,
		},
	}

	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	return &corev1alpha1.ResourceTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Spec: spec,
	}, key
}
