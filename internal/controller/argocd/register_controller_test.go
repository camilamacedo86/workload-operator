/*
Copyright 2023 Camila Macedo.

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

package argocd

import (
	"context"
	"fmt"
	"time"

	"github.com/workload-operator/internal/argocd/mocks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterapiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argocdv1beta1 "github.com/workload-operator/api/argocd/v1beta1"
	"github.com/workload-operator/internal/status"
)

var _ = Describe("Register controller", func() {
	Context("Register controller mocks", func() {

		const RegisterNamespace = "mocks-register"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      RegisterNamespace,
				Namespace: RegisterNamespace,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: RegisterNamespace, Namespace: RegisterNamespace}
		registerCR := &argocdv1beta1.Register{}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			By("creating the custom resource for the Cluster to emulate values in the namespace")
			err = k8sClient.Get(ctx, typeNamespaceName, registerCR)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				cluster := &clusterapiv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      RegisterNamespace,
						Namespace: namespace.Name,
					},
					Spec: clusterapiv1.ClusterSpec{
						ControlPlaneEndpoint: clusterapiv1.APIEndpoint{Host: "mocks", Port: 80},
					},
				}

				err = k8sClient.Create(ctx, cluster)
				Expect(err).To(Not(HaveOccurred()))
			}

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      RegisterNamespace,
					Namespace: RegisterNamespace, // Make sure to use the correct namespace
				},
				Data: map[string][]byte{
					"kubeconfig": []byte(mocks.MockKubeConfig), // Insert your kubeconfig data here
				},
			}

			err = k8sClient.Create(ctx, secret)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("removing the custom resource for the Cluster")
			found := &clusterapiv1.Cluster{}
			err := k8sClient.Get(ctx, typeNamespaceName, found)
			Expect(err).To(Not(HaveOccurred()))

			Eventually(func() error {
				return k8sClient.Delete(ctx, found)
			}, 2*time.Minute, time.Second).Should(Succeed())

			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for Cluster", func() {
			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &clusterapiv1.Cluster{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			registerReconciler := &RegisterReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := registerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking the latest Status Condition added to the Register instance")
			Eventually(func() error {
				if registerCR.Status.Conditions != nil && len(registerCR.Status.Conditions) != 0 {
					latestStatusCondition := registerCR.Status.Conditions[len(registerCR.Status.Conditions)-1]
					if latestStatusCondition.Type != status.ConditionAvailable {
						return fmt.Errorf("latest status condition added to the Register instance is not as expected")
					}
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
