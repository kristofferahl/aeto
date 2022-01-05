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

	corev1alpha1 "github.com/kristofferahl/aeto/api/v1alpha1"
	testing "github.com/kristofferahl/aeto/internal/pkg/testing"
)

var _ = Describe("Tenant Controller", func() {

	const timeout = time.Second * 30
	const interval = time.Second * 1

	const resourceName = "test-tenant"
	const namespace = "default"

	Context("Tenant with valid spec", func() {
		It("Should handle reconcile correctly", func() {
			toCreate, key := testing.NewTenant(namespace, resourceName)

			By("Creating the Tenant successfully")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			time.Sleep(time.Second * 5)

			fetched := &corev1alpha1.Tenant{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), key, fetched)
				return fetched != nil
			}, timeout, interval).Should(BeTrue())

			Expect(fetched.Generation).To(BeEquivalentTo(1))
			Expect(fetched.Spec.Name).To(Equal("Tenant name"))
		})
	})
})
