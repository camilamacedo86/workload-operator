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
	"testing"

	"github.com/workload-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// namespace which will be used to test the operator
const (
	nameWorkloadCluster   = "workload-cluster"
	nameManagementCluster = "management-cluster"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	fmt.Fprintf(GinkgoWriter, "Starting Workload Operator E2E Tests suite\n")
	RunSpecs(t, "Workload Operator e2e suite")
}

// BeforeSuite run before any specs are run to perform the required actions for all e2e Go tests.
var _ = BeforeSuite(func() {
	By("creating management cluster")
	err := utils.CreateKindClusterWith(nameManagementCluster)
	Expect(err).To(Not(HaveOccurred()))

	By("installing ArgoCD")
	err = utils.InstallArgoCD()
	Expect(err).To(Not(HaveOccurred()))

	By("exposing ArgoCD API")
	err = utils.ExposeArgoCDAPI()
	Expect(err).To(Not(HaveOccurred()))

	By("creating workload cluster")
	err = utils.CreateKindClusterWith(nameWorkloadCluster)
	Expect(err).To(Not(HaveOccurred()))

	By("setting up context as management cluster")
	err = utils.SetKubeContext(nameManagementCluster)
	Expect(err).To(Not(HaveOccurred()))
})

// AfterSuite run after all the specs have run, regardless of whether any tests have failed to ensures that
// all be cleaned up
var _ = AfterSuite(func() {
	By("delete namespace for the workload cluster")
	cmd := exec.Command("kubectl", "delete", "ns", testNamespaceForWorkloadCluster)
	_, _ = utils.Run(cmd)

	By("deleting workload cluster")
	_ = utils.DeleteKindClusterWith(nameWorkloadCluster)

	By("uninstalling ArgoCD")
	utils.UninstallArgoCD()

	By("removing management cluster")
	err := utils.DeleteKindClusterWith(nameManagementCluster)
	Expect(err).To(Not(HaveOccurred()))
})
