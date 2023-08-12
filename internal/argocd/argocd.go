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

// Package argocd provides utilities for interacting with ArgoCD.
// It includes functions to manage and control clusters within ArgoCD,
// such as registering, unregistering, and validating clusters.
// Configuration and authentication details can be managed within the package,
// allowing seamless integration with ArgoCD APIs.
// More info: https://cd.apps.argoproj.io/swagger-ui
package argocd

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/clientcmd"

	clusterapiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// NamespaceEnvVar store the name of the envvar used to provide the Namespace where ArgoCD is
	// deployed on cluster
	NamespaceEnvVar = "ARGOCD_NAMESPACE"

	// SecretNameEnvVar store the name of the envvar used to provide the SecretName used to get
	// the token to authenticate within to Argo API
	SecretNameEnvVar = "ARGOCD_SECRET_NAME"

	// APIEndpointEnvVar store the name of the envvar used to provide the API Endpoint
	APIEndpointEnvVar = "ARGOAPI_ENDPOINT"

	defaultSecretName      = "argocd-secret"
	defaultNamespace       = "argocd"
	defaultArgoAPIEndpoint = "https://argocd-api.example.com"
)

// APIManager stores the required information to interact with the ArgoCD API.
type APIManager struct {
	Token      string          // The ArgoCD API token
	Client     client.Client   // Kubernetes client
	Ctx        context.Context // Context for the operations
	Log        logr.Logger     // Logger for the manager
	Server     string          // Server endpoint for ArgoCD
	Name       string          // Name of the cluster
	KubeConfig []byte          // Kubeconfig content in bytes
	Endpoint   string          // ArgoCD API endpoint
}

// NewAPIManagerWithCluster returns the Manager to allow to perform operations against the ArgoCD API.
func NewAPIManagerWithCluster(ctx context.Context, client client.Client, log logr.Logger,
	clusterAPI *clusterapiv1.Cluster, kubeConfig []byte) (*APIManager, error) {

	argoAPIEndpoint, exists := os.LookupEnv(APIEndpointEnvVar)
	if !exists {
		log.Info(fmt.Sprintf("Argo API Endpoint is not provided via Manager ENV VAR, "+
			"using default value (%s)", defaultArgoAPIEndpoint))
		argoAPIEndpoint = defaultArgoAPIEndpoint
	}

	newArgo := &APIManager{
		Client: client,
		Ctx:    ctx,
		Log:    log,
		Server: clusterAPI.Spec.ControlPlaneEndpoint.Host + ":" +
			strconv.Itoa(int(clusterAPI.Spec.ControlPlaneEndpoint.Port)),
		Name:       clusterAPI.Name,
		KubeConfig: kubeConfig,
		Endpoint:   argoAPIEndpoint,
	}
	err := newArgo.setBareToken()

	return newArgo, err
}

// setBareToken retrieves the ArgoCD API token from its namespace and sets it in the struct.
func (a *APIManager) setBareToken() error {

	argocdNamespace, exists := os.LookupEnv(NamespaceEnvVar)
	if !exists {
		a.Log.Info(fmt.Sprintf("Argo Instance Namespace is not provided via Manager ENV VAR, "+
			"using default value (%s)", defaultNamespace))
		argocdNamespace = defaultNamespace
	}

	argocdSecretName, exists := os.LookupEnv(SecretNameEnvVar)
	if !exists {
		a.Log.Info(fmt.Sprintf("Argo Instance Secret Name is not provided via Manager ENV VAR, "+
			"using default value (%s)", defaultSecretName))
		argocdSecretName = defaultSecretName
	}

	secret := &v1.Secret{}
	if err := a.Client.Get(a.Ctx, client.ObjectKey{
		Namespace: argocdNamespace,
		Name:      argocdSecretName,
	}, secret); err != nil {
		return fmt.Errorf("error fetching secret: %w", err)
	}

	// Decode the token
	tokenBase64, ok := secret.Data["admin.password"]
	if !ok {
		return fmt.Errorf("admin.password not found in secret")
	}

	token, err := base64.StdEncoding.DecodeString(string(tokenBase64))
	if err != nil {
		return err
	}

	a.Token = string(token)
	return nil
}

// ValidateKubeConfigForClusterAPI checks if the kubeconfig retrieved is valid for the cluster.
func (a *APIManager) ValidateKubeConfigForClusterAPI() error {
	_, err := clientcmd.Load(a.KubeConfig)
	if err != nil {
		return fmt.Errorf("error loading kubeconfig: %w", err)
	}

	// TODO: Add further checks

	return nil
}

// RegisterCluster registers the Cluster to the ArgoCD.
func (a *APIManager) RegisterCluster() error {
	if err := a.ValidateKubeConfigForClusterAPI(); err != nil {
		return err
	}

	argocdCluster := map[string]interface{}{
		"server":     a.Server,
		"name":       a.Name,
		"kubeconfig": a.KubeConfig,
		"config": map[string]interface{}{
			"bearerToken": a.Token,
		},
	}

	payload, err := json.Marshal(argocdCluster)
	if err != nil {
		return fmt.Errorf("error marshalling payload: %w", err)
	}

	url := a.Endpoint + "/api/v1/clusters"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.Token)

	client := &http.Client{
		Timeout: time.Second * 30,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer func() {
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			a.Log.Error(err, "Error reading response body")
		}
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error registering cluster, status: %s", resp.Status)
	}

	return nil
}

// IsClusterRegistered returns true when registered or an error if face issues to do the check.
func (a *APIManager) IsClusterRegistered() (bool, error) {
	// TODO: Implement check
	return false, nil
}

// CheckRegistration returns an error when issues were found into the registration.
func (a *APIManager) CheckRegistration() error {
	// TODO: Implement check
	return nil
}

// UnRegisterCluster unregisters a cluster from the ArgoCD instance or returns an error for failure scenarios.
func (a *APIManager) UnRegisterCluster() error {
	// TODO: Implement request to unregisterCluster
	return nil
}
