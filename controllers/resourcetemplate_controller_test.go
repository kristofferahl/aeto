/*
Copyright 2021.

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

package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/kristofferahl/aeto/api/v1alpha1"
	"github.com/kristofferahl/aeto/internal/pkg/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ResourceTemplate Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	const resourceName = "test-resource-template"
	const namespace = "default"

	Context("ResourceTemplate with valid spec", func() {
		It("Should handle reconcile correctly", func() {
			spec := corev1alpha1.ResourceTemplateSpec{
				Raw: []string{
					`
					apiVersion: v1
					kind: Namespace
					metadata:
					name: {{ .Namespace }}
					labels:
					app.kubernetes.io/tenant: "{{ .Key }}"
					`,
				},

				Manifests: []corev1alpha1.EmbeddedResource{
					{
						RawExtension: runtime.RawExtension{
							Raw: []byte(testing.MustConvert(&corev1.Pod{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test",
								},
								Spec: corev1.PodSpec{},
							})),
						},
					},
				},
			}

			key := types.NamespacedName{
				Name:      resourceName,
				Namespace: namespace,
			}

			toCreate := &corev1alpha1.ResourceTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: spec,
			}

			By("Creating the ResourceTemplate successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetched := &corev1alpha1.ResourceTemplate{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), key, fetched)
				return fetched != nil
			}, timeout, interval).Should(BeTrue())

			Expect(fetched.Generation).To(BeEquivalentTo(1))
			Expect(len(fetched.Spec.Manifests)).To(BeEquivalentTo(1))
			Expect(len(fetched.Spec.Raw)).To(BeEquivalentTo(1))
		})
	})
})
