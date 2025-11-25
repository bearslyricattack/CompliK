/*
Copyright 2025 gitlayzer.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "github.com/bearslyricattack/CompliK/block-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("BlockRequest Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		blockrequest := &apiv1.BlockRequest{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind BlockRequest")
			err := k8sClient.Get(ctx, typeNamespacedName, blockrequest)
			if err != nil && errors.IsNotFound(err) {
				resource := &apiv1.BlockRequest{ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
					Spec: apiv1.BlockRequestSpec{
						NamespaceNames: []string{"test-ns"},
						Action:         "locked",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &apiv1.BlockRequest{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance BlockRequest")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &BlockRequestReconciler{
				Client:           k8sClient,
				NonCachingClient: k8sClient,
				Scheme:           k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When reconciling a resource with namespaceSelector", func() {
		const resourceName = "test-resource-selector"
		const nsName = "test-ns-selector"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a namespace with a label")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   nsName,
					Labels: map[string]string{"test-label": "true"},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("creating the custom resource for the Kind BlockRequest with namespaceSelector")
			resource := &apiv1.BlockRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: apiv1.BlockRequestSpec{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"test-label": "true"},
					},
					Action: "locked",
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("deleting the BlockRequest resource")
			resource := &apiv1.BlockRequest{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("deleting the namespace")
			ns := &corev1.Namespace{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, ns)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
		})

		It("should successfully reconcile the resource and label the namespace", func() {
			By("Reconciling the created resource")
			controllerReconciler := &BlockRequestReconciler{
				Client:           k8sClient,
				NonCachingClient: k8sClient,
				Scheme:           k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("checking if the namespace has the correct label")
			var updatedNs corev1.Namespace
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nsName}, &updatedNs)
				if err != nil {
					return false
				}
				return updatedNs.Labels["clawcloud.run/status"] == "locked"
			}, "5s", "1s").Should(BeTrue())
		})
	})
})
