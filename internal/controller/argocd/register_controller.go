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

// Package argocd responsible to centralize the controller logic for
// operations which involves ArgoCD integrations
package argocd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterapiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	argocdv1beta1 "github.com/workload-operator/api/argocd/v1beta1"
	"github.com/workload-operator/internal/argocd"
	"github.com/workload-operator/internal/status"
)

// RegisterReconciler reconciles a Register object
type RegisterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Log      logr.Logger
}

const registerCRFinalizer = "argocd.register.workload.com/finalizer"

//+kubebuilder:rbac:groups=argocd.workload.com,resources=instances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=argocd.workload.com,resources=instances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=argocd.workload.com,resources=instances/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile will reconcile Clusters resources from the API clusters.cluster.x-k8s.io since
// then represent a Workload Cluster and either Register Instances created and managed into
// this reconciliation due to the fact its purpose is to ensure the Workload Cluster registration
// within ArgoCD in the Management Cluster.
func (r *RegisterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)

	clusterAPI := &clusterapiv1.Cluster{}
	RegisterCR := &argocdv1beta1.Register{}
	if err := r.Get(ctx, req.NamespacedName, clusterAPI); err != nil {
		if !apierrors.IsNotFound(err) {
			r.Log.Error(err, "Failed to get Cluster CR")
			return ctrl.Result{}, err
		}
		// If the namespace no longer has the Cluster CR then, it means that the instance was deleted
		// Therefore, we must check if we have a Register CR exist into the namespace
		// since it represents the ArgoCD Registration within the Cluster Workload
		if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
			if apierrors.IsNotFound(err) {
				// If the RegisterCR is not found then we can ignore and stop the reconciliation
				r.Log.Info("Register resource not found. Ignoring since object must be deleted")
				return ctrl.Result{}, nil
			}
			r.Log.Error(err, "Failed to get RegisterCR")
			return ctrl.Result{}, err
		}

		// If Register CR exist and is not marked to be deleted then we will mark it
		if isMarkedToBeDeleted := RegisterCR.GetDeletionTimestamp() != nil; !isMarkedToBeDeleted {
			RegisterCR.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
			err := r.Client.Update(ctx, RegisterCR)
			if err != nil {
				r.Log.Error(err, "Failed to set Deletion Timestamp on Register")
				return ctrl.Result{}, err
			}
		}
	}

	// Check if Register exist, if not create
	if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
		if !apierrors.IsNotFound(err) {
			r.Log.Error(err, "Failed to fetch Register for ArgoCD")
			return ctrl.Result{}, err
		}
		if err = r.createRegisterCR(ctx, clusterAPI, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to create Register Instance CR")
			return ctrl.Result{}, err
		}
		if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to fetch Register Instance CR")
			return ctrl.Result{}, err
		}
	}

	// Gathering the data, validate and create a argoCDAPIManager to allow us to perform operations
	// using ArgoCD API
	argoCDAPIManager, err := r.handleIntegrationWithArgoCDAPI(ctx, req, RegisterCR, clusterAPI)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check if RegisterCR is marked to be deleted, if yes then handle finalization
	if isMarkedToBeDeleted := RegisterCR.GetDeletionTimestamp() != nil; isMarkedToBeDeleted {
		if err := r.handleFinalizer(ctx, RegisterCR, req, argoCDAPIManager); err != nil {
			return ctrl.Result{}, err
		}
		// Finalize reconciliation since the Register was marked to be deleted and
		// all required operations to allow to do so were completed successfully
		return ctrl.Result{}, nil
	}

	if err := r.handleClusterRegistration(ctx, req, argoCDAPIManager, RegisterCR); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RegisterReconciler) handleIntegrationWithArgoCDAPI(ctx context.Context, req ctrl.Request,
	RegisterCR *argocdv1beta1.Register, clusterAPI *clusterapiv1.Cluster) (*argocd.APIManager, error) {
	kubeconfigContent, err := r.getClusterKubeConfigFromSecret(ctx, req)
	if err != nil {
		r.Log.Error(err, "Failed to get KubeConfigFromSecret")
		if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to get RegisterCR")
			return nil, err
		}
		meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
			Status: metav1.ConditionTrue, Reason: "Error",
			Message: fmt.Sprintf("Unable to gathering kubeConfig: %s", err)})
		if err := r.Status().Update(ctx, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to update Register status")
			return nil, err
		}
		return nil, err
	}

	// Create the APIManager so that is possible to interact with ArgoCD API
	argoCDAPIManager, err := argocd.NewAPIManagerWithCluster(ctx, r.Client, r.Log, clusterAPI, kubeconfigContent)
	if err != nil {
		r.Log.Error(err, "Failed to gathering pre-requirements to connect with ArgoCD")
		if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to get RegisterCR")
			return nil, err
		}
		meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
			Status: metav1.ConditionTrue, Reason: "Error",
			Message: fmt.Sprintf("Unable to gathering pre-requirements to connect with ArgoCD: %s", err)})
		if err := r.Status().Update(ctx, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to update Register status")
			return nil, err
		}
	}
	return argoCDAPIManager, nil
}

// handleClusterRegistration  will verify if the Cluster is or not registered, if not register it
func (r *RegisterReconciler) handleClusterRegistration(ctx context.Context, req ctrl.Request,
	argoCDManager *argocd.APIManager, RegisterCR *argocdv1beta1.Register) error {

	isClusterRegistered, err := argoCDManager.IsClusterRegistered()
	if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
		r.Log.Error(err, "Failed to get RegisterCR")
		return err
	}
	if err != nil {
		r.Log.Error(err, "Failed to Check Cluster Registration")
		meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
			Status: metav1.ConditionTrue, Reason: "Error",
			Message: fmt.Sprintf("Unable to verify Cluster Registration: %s", err)})
		if err := r.Status().Update(ctx, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to update Register status")
			return err
		}
	}

	if !isClusterRegistered {
		if err := argoCDManager.RegisterCluster(); err != nil {
			r.Log.Error(err, "Failed to Register Cluster into ArgoCD")
			meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
				Status: metav1.ConditionTrue, Reason: "Error",
				Message: fmt.Sprintf("Unable to register Cluster into ArgoCD: %s", err)})
			if err := r.Status().Update(ctx, RegisterCR); err != nil {
				r.Log.Error(err, "Failed to update Register status")
				return err
			}
		}
	}

	meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionAvailable,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: "Cluster is Registered"})
	if err := r.Status().Update(ctx, RegisterCR); err != nil {
		r.Log.Error(err, "Failed to update Register status")
		return err
	}
	return nil
}

func (r *RegisterReconciler) createRegisterCR(ctx context.Context, clusterAPI *clusterapiv1.Cluster,
	RegisterCR *argocdv1beta1.Register) error {
	// Create the Register which will represent the registration with ArgoCD in the cluster
	newRegister, err := r.generateRegisterCR(clusterAPI)
	if err != nil {
		return fmt.Errorf("failed to generate Register CR: %w", err)
	}

	// Let's add here a status "Downgrade" to define that this resource begin its process to be terminated.
	meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionProgressing,
		Status: metav1.ConditionTrue, Reason: "Creating Register",
		Message: "Preparing to Register Cluster with ArgoCD"})

	// Create the Register CR in the cluster
	if err := r.Client.Create(ctx, newRegister); err != nil {
		return fmt.Errorf("failed to create Register CR: %w", err)
	}
	return nil
}

// handleFinalizer will handle the finalization of the Register CR to allow kubernetes API delete it
func (r *RegisterReconciler) handleFinalizer(ctx context.Context, RegisterCR *argocdv1beta1.Register, req ctrl.Request,
	argoCDManager *argocd.APIManager) error {
	if controllerutil.ContainsFinalizer(RegisterCR, registerCRFinalizer) {
		r.Log.Info("Performing Finalizer Operations for RegisterCR before delete CR")
		meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
			Status: metav1.ConditionTrue, Reason: "Finalizing",
			Message: "Performing finalizer operations to delete Register"})
		if err := r.Status().Update(ctx, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to update Register status")
			return err
		}
		if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to re-fetch RegisterCR")
			return err
		}

		// Perform all operations required before remove the finalizer and allow
		// the Kubernetes API to remove the custom resource.
		if err := r.doFinalizerOperations(RegisterCR, argoCDManager); err != nil {
			meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Error to perform required operations: %s", err)})
			if err := r.Status().Update(ctx, RegisterCR); err != nil {
				r.Log.Error(err, "Failed to update Register status")
				return err
			}
			return err
		}

		meta.SetStatusCondition(&RegisterCR.Status.Conditions, metav1.Condition{Type: status.ConditionDegraded,
			Status: metav1.ConditionTrue, Reason: "Finalizing",
			Message: "Cluster is unregister successfully accomplished"})
		if err := r.Status().Update(ctx, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to update Register status")
			return err
		}

		r.Log.Info("Removing Finalizer for RegisterCR after successfully perform the operations")
		if err := r.Get(ctx, req.NamespacedName, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to re-fetch RegisterCR")
			return err
		}
		if ok := controllerutil.RemoveFinalizer(RegisterCR, registerCRFinalizer); !ok {
			r.Log.Error(errors.New("failed to remove finalizer from Register CR"), "Unable to finalize:")
			return nil
		}
		if err := r.Update(ctx, RegisterCR); err != nil {
			r.Log.Error(err, "Failed to update Register to remove finalizer")
			return err
		}
	}
	return nil
}

// generateRegisterCR will return the Register Instance to represent on cluster the registration within the ArgoCD API
func (r *RegisterReconciler) generateRegisterCR(clusterAPI *clusterapiv1.Cluster) (*argocdv1beta1.Register, error) {
	// Define the Register Resource
	newRegister := &argocdv1beta1.Register{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterAPI.Name,
			Namespace: clusterAPI.Namespace,
		},
	}

	// Set the owner reference for garbage collection if needed
	return newRegister, controllerutil.SetOwnerReference(clusterAPI, newRegister, r.Scheme)
}

// getClusterKubeConfigFromSecret will retrieve the kubeConfig stored in the secret of the current
// namespace. The Cluster Workload kubeconfig is stored in a secret into the namespace
// therefore we will retrieve it within the assumption that each namespace has only one secret.
// However, if that is not true, then we must filter ideally by labels or by name
func (r *RegisterReconciler) getClusterKubeConfigFromSecret(ctx context.Context, req ctrl.Request) ([]byte, error) {
	// Fetch the associated kubeconfig secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		return nil, err
	}

	// Extract the kubeconfig
	kubeconfig, exists := secret.Data["kubeconfig"] // or "kubeconfig", depending on the actual key
	if !exists {
		return nil, fmt.Errorf("kubeconfig not found in secret")
	}
	return kubeconfig, nil
}

// doFinalizerOperations will perform the required operations before delete the CR.
func (r *RegisterReconciler) doFinalizerOperations(cr *argocdv1beta1.Register,
	argoCDManager *argocd.APIManager) error {
	if err := argoCDManager.UnRegisterCluster(); err != nil {
		r.Log.Error(err, "Failed to Unregister Cluster from ArgoCD")
		return err
	}

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Register CR %s from the namespace %s will be deleted.",
			cr.Namespace,
			cr.Name,
		))

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RegisterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Owns(&argocdv1beta1.Register{}).
		For(&clusterapiv1.Cluster{}).
		Owns(&argocdv1beta1.Register{}).
		Complete(r)
}
