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

package sustainability

import (
	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ = Describe("Deployment Resource", func() {
	Describe("SavingsPolicy", func() {
		Describe("has deployment targets?", func() {
			Context("with deployment targets in policy", func() {
				sp := sustainabilityv1alpha1.SavingsPolicy{
					Spec: sustainabilityv1alpha1.SavingsPolicySpec{
						Targets: []sustainabilityv1alpha1.SavingsPolicyTarget{
							{
								Kind:       DeploymentResourceKind,
								ApiVersion: DeploymentResourceApiVersion,
								Ignore:     true,
							},
						},
					},
				}
				result := hasDeploymentTargets(sp)

				It("should return true", func() {
					Expect(result).To(BeTrue())
				})
			})

			Context("with no deployment targets", func() {
				sp := sustainabilityv1alpha1.SavingsPolicy{
					Spec: sustainabilityv1alpha1.SavingsPolicySpec{
						Targets: []sustainabilityv1alpha1.SavingsPolicyTarget{
							{
								Kind:       "Foobar",
								ApiVersion: DeploymentResourceApiVersion,
								Ignore:     true,
							},
						},
					},
				}
				result := hasDeploymentTargets(sp)

				It("should return false", func() {
					Expect(result).To(BeFalse())
				})
			})
		})

		Describe("ignore deployment target?", func() {
			It("should return false when target matches all deployments", func() {
				sp := sustainabilityv1alpha1.SavingsPolicy{
					Spec: sustainabilityv1alpha1.SavingsPolicySpec{
						Targets: []sustainabilityv1alpha1.SavingsPolicyTarget{
							{
								Kind:       DeploymentResourceKind,
								ApiVersion: DeploymentResourceApiVersion,
								Ignore:     false,
							},
						},
					},
				}

				d := appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						Kind:       DeploymentResourceKind,
						APIVersion: DeploymentResourceApiVersion,
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "foobar",
					},
				}

				result := ignoreDeploymentTarget(sp, d)

				Expect(result).To(BeFalse())
			})

			It("should return true when target matches all deployments but ignore is set to true", func() {
				sp := sustainabilityv1alpha1.SavingsPolicy{
					Spec: sustainabilityv1alpha1.SavingsPolicySpec{
						Targets: []sustainabilityv1alpha1.SavingsPolicyTarget{
							{
								Kind:       DeploymentResourceKind,
								ApiVersion: DeploymentResourceApiVersion,
								Ignore:     true,
							},
						},
					},
				}

				d := appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						Kind:       DeploymentResourceKind,
						APIVersion: DeploymentResourceApiVersion,
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "foobar",
					},
				}

				result := ignoreDeploymentTarget(sp, d)

				Expect(result).To(BeTrue())
			})

			It("should return false when target matches deployment explicitly but ignore is set to false", func() {
				name := "foobar"

				sp := sustainabilityv1alpha1.SavingsPolicy{
					Spec: sustainabilityv1alpha1.SavingsPolicySpec{
						Targets: []sustainabilityv1alpha1.SavingsPolicyTarget{
							{
								Kind:       DeploymentResourceKind,
								ApiVersion: DeploymentResourceApiVersion,
								Name:       name,
								Ignore:     false,
							},
						},
					},
				}

				d := appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						Kind:       DeploymentResourceKind,
						APIVersion: DeploymentResourceApiVersion,
					},
					ObjectMeta: v1.ObjectMeta{
						Name: name,
					},
				}

				result := ignoreDeploymentTarget(sp, d)

				Expect(result).To(BeFalse())
			})

			It("should return true when target matches deployment explicitly but ignore is set to true", func() {
				name := "foobar"

				sp := sustainabilityv1alpha1.SavingsPolicy{
					Spec: sustainabilityv1alpha1.SavingsPolicySpec{
						Targets: []sustainabilityv1alpha1.SavingsPolicyTarget{
							{
								Kind:       DeploymentResourceKind,
								ApiVersion: DeploymentResourceApiVersion,
								Name:       name,
								Ignore:     true,
							},
						},
					},
				}

				d := appsv1.Deployment{
					TypeMeta: v1.TypeMeta{
						Kind:       DeploymentResourceKind,
						APIVersion: DeploymentResourceApiVersion,
					},
					ObjectMeta: v1.ObjectMeta{
						Name: name,
					},
				}

				result := ignoreDeploymentTarget(sp, d)

				Expect(result).To(BeTrue())
			})
		})
	})
})
