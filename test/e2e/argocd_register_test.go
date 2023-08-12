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

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	argocdv1beta1 "github.com/workload-operator/api/argocd/v1beta1"
	"github.com/workload-operator/internal/status"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	clusterapiv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	//nolint:golint
	//nolint:revive
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/workload-operator/test/utils"
)

const testNamespaceForWorkloadCluster = "test-workload-cluster"
const operatorNamespace = "workload-operator-system"

var _ = Describe("ArgoCD", Ordered, func() {
	Context("Registration", func() {
		It("should run successfully", func() {
			var controllerPodName string
			var err error
			var operatorImage = "example.com/workload-operator:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", operatorImage))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("setting up context as management cluster")
			err = utils.SetKubeContext(nameManagementCluster)
			Expect(err).To(Not(HaveOccurred()))

			By("loading the Operator image on Kind")
			_ = utils.LoadImageToKindClusterWithName(operatorImage, nameManagementCluster)
			Expect(err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			Expect(err).To(Not(HaveOccurred()))

			By("deploying the operator")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", operatorImage))
			_, err = utils.Run(cmd)
			Expect(err).To(Not(HaveOccurred()))

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name
				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}{{ if not .metadata.deletionTimestamp }}{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", operatorNamespace,
				)
				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", operatorNamespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())
		})

		It("should trigger the reconciliation and Register to be Available", func() {
			By("setting up context as management cluster")
			err := utils.SetKubeContext(nameManagementCluster)
			Expect(err).To(Not(HaveOccurred()))

			By("creating namespace for the workload cluster")
			cmd := exec.Command("kubectl", "create", "ns", testNamespaceForWorkloadCluster)
			_, err = utils.Run(cmd)
			Expect(err).To(Not(HaveOccurred()))

			By("creating kubeconfig Secret for the workload cluster")
			secret, err := createKubeconfigSecret(nameWorkloadCluster, testNamespaceForWorkloadCluster)
			Expect(err).To(Not(HaveOccurred()))

			By("marshal the Secret into YAML")
			yamlBytes, err := yaml.Marshal(secret)
			Expect(err).To(Not(HaveOccurred()))

			By("creating Secret to hold kubeconfig")
			cmd = exec.Command("kubectl", "-n", testNamespaceForWorkloadCluster, "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(string(yamlBytes))
			_, err = cmd.CombinedOutput()
			Expect(err).To(Not(HaveOccurred()))

			By("creating Cluster API for the workload cluster")
			clusterAPI, err := createClusterAPICluster(nameWorkloadCluster)
			Expect(err).To(Not(HaveOccurred()))
			Expect(clusterAPI).ToNot(BeNil())

			By("marshal the struct into YAML")
			yamlBytes, err = yaml.Marshal(clusterAPI)
			Expect(err).To(Not(HaveOccurred()))

			By("Creating Cluster CR to trigger reconcile")
			cmd = exec.Command("kubectl", "-n", testNamespaceForWorkloadCluster, "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(string(yamlBytes))
			_, err = cmd.CombinedOutput()
			Expect(err).To(Not(HaveOccurred()))

			By("Checking the latest Status Condition added to the Register instance")
			Eventually(func() error {
				registerCR, err := getRegisterCR(testNamespaceForWorkloadCluster, clusterAPI.Name)
				if err != nil {
					return err
				}

				if registerCR.Status.Conditions != nil && len(registerCR.Status.Conditions) != 0 {
					latestStatusCondition := registerCR.Status.Conditions[len(registerCR.Status.Conditions)-1]
					if latestStatusCondition.Type != status.ConditionAvailable {
						return fmt.Errorf("latest status condition added to the " +
							"Register instance is not as expected")
					}
				}
				return nil
			}, 2*time.Minute, time.Second).Should(Succeed())

		})
	})
})

// createClusterAPICluster using the data of the workload cluster
func createClusterAPICluster(clusterName string) (*clusterapiv1.Cluster, error) {
	// Get the Kubernetes API server endpoint for the workload cluster
	cmd := exec.Command("kubectl", "config", "view", "-o",
		"jsonpath={.clusters[?(@.name==\"kind-"+clusterName+"\")].cluster.server}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get API server endpoint for cluster %s: %s\n%s",
			clusterName, err, string(output))
	}

	// Extract the Host and Port from the API server endpoint
	endpoint := strings.Trim(string(output), "\n")
	hostAndPort := strings.Split(strings.TrimPrefix(endpoint, "https://"), ":")
	if len(hostAndPort) != 2 {
		return nil, fmt.Errorf("invalid API server endpoint format: %s", endpoint)
	}

	// Create the Cluster API Cluster object
	cluster := &clusterapiv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: testNamespaceForWorkloadCluster,
		},
		Spec: clusterapiv1.ClusterSpec{
			ControlPlaneEndpoint: clusterapiv1.APIEndpoint{
				Host: hostAndPort[0],
				Port: 6443, // Assuming standard API server port
			},
		},
	}

	return cluster, nil
}

func createKubeconfigSecret(clusterName string, namespace string) (*v1.Secret, error) {
	// Retrieve the kubeconfig for the given cluster
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
	kubeconfigBytes, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig for cluster %s: %v", clusterName, err)
	}

	// Create the Secret object
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName + "-kubeconfig",
			Namespace: namespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": kubeconfigBytes,
		},
	}

	return secret, nil
}

func getRegisterCR(namespace string, name string) (*argocdv1beta1.Register, error) {
	cmd := exec.Command("kubectl", "get", "register", name, "-n", namespace, "-o", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get Register CR: %s\n%s", err, string(output))
	}

	var registerCR argocdv1beta1.Register
	if err := json.Unmarshal(output, &registerCR); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Register CR: %s", err)
	}

	return &registerCR, nil
}
