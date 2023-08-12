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

// Package utils has utilities useful for the e2e test
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
)

const (
	argoCDInstallURL = "https://raw.githubusercontent.com/argoproj/argo-cd/release-2.8/manifests/install.yaml"
)

func warnError(err error) {
	fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// InstallArgoCD install ArgoCD in the cluster
func InstallArgoCD() error {
	cmd := exec.Command("kubectl", "create", "namespace", "argocd")
	output, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("unable to create argocd namespace. Command (%s) "+
			"failed with error: (%v) %s", cmd, err, string(output))
	}

	cmd = exec.Command("kubectl", "apply", "-n", "argocd", "-f", argoCDInstallURL)
	output, err = Run(cmd)
	if err != nil {
		return fmt.Errorf("unable to create argocd namespace. Command (%s) "+
			"failed with error: (%v) %s", cmd, err, string(output))
	}
	return nil
}

// ExposeArgoCDAPI will expose the API to allow interactions within
func ExposeArgoCDAPI() error {
	cmd := exec.Command("kubectl", "patch", "svc", "argocd-server", "-n",
		"argocd", "-p", `{"spec": {"type": "LoadBalancer"}}`)
	output, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("unable to patch argocd-server service. "+
			"Command (%s) failed with error: (%v) %s", cmd, err, string(output))
	}
	return nil
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir
	fmt.Fprintf(GinkgoWriter, "running dir: %s\n", cmd.Dir)

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	fmt.Fprintf(GinkgoWriter, "running: %s\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}

	return output, nil
}

// UninstallArgoCD uninstalls ArgoCD
func UninstallArgoCD() {
	cmd := exec.Command("kubectl", "delete", "namespace", "argocd")
	_, err := Run(cmd)
	if err != nil {
		warnError(err)
	}
}

// CreateKindClusterWith will create a kind cluster with the name informed
func CreateKindClusterWith(name string) error {
	kindOptions := []string{"create", "cluster", "--name", name}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to create management cluster: %w", err)
	}
	return nil
}

// DeleteKindClusterWith will create a kind cluster with the name informed
func DeleteKindClusterWith(name string) error {
	kindOptions := []string{"delete", "cluster", "--name", name}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to create management cluster: %w", err)
	}
	return nil
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(imageName, clusterName string) error {
	kindOptions := []string{"load", "docker-image", imageName, "--name", clusterName}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

// SetKubeContext will setup the context that should be use
func SetKubeContext(clusterName string) error {
	cmd := exec.Command("kubectl", "config", "use-context", "kind-"+clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set kube context to %s: %s\n%s", clusterName, err, string(output))
	}
	return nil
}
