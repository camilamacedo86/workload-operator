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
	"encoding/base64"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/workload-operator/internal/argocd/mocks"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterapiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("ArgoCD APIManager", func() {
	Context("APIManager creation", func() {
		argoNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultNamespace,
				Namespace: defaultNamespace,
			},
		}

		ctx := context.Background()
		var testLog logr.Logger
		var secret corev1.Secret

		BeforeEach(func() {
			By("creating Argo namespace")
			err := k8sClient.Create(ctx, argoNs)
			Expect(err).To(Not(HaveOccurred()))

			By(" creating Argo the secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultSecretName, // or "argocd-secret"
					Namespace: defaultNamespace,  // or "argocd"
				},
				Data: map[string][]byte{
					"admin.password": []byte(base64.StdEncoding.EncodeToString([]byte("token-test"))),
				},
			}
			err = k8sClient.Create(ctx, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("cleaning up Argo Mock secret")
			_ = k8sClient.Delete(ctx, &secret)

			By("deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, argoNs)
		})

		It("should create a new APIManager with the expected values", func() {
			By("creating a new cluster instance")
			cluster := &clusterapiv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: clusterapiv1.ClusterSpec{
					ControlPlaneEndpoint: clusterapiv1.APIEndpoint{Host: "Host", Port: 80},
				},
			}

			By("creating a new APIManager instance with the cluster")
			apiManager, err := NewAPIManagerWithCluster(ctx, k8sClient, testLog, cluster, []byte(mocks.MockKubeConfig))
			Expect(err).To(Not(HaveOccurred()))
			Expect(apiManager).To(Not(BeNil()))

			By("checking expected results")
			Expect(apiManager.Endpoint).To(Equal(defaultArgoAPIEndpoint))
			Expect(apiManager.Token).To(Not(BeNil()))
			Expect(apiManager.Name).To(Equal("test"))
			Expect(apiManager.KubeConfig).To(Equal([]byte(mocks.MockKubeConfig)))
			Expect(apiManager.Server).To(Equal("Host:80"))
		})
	})
})
