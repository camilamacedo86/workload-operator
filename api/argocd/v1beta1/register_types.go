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

// Package v1beta1 defines the APIs that represents operations on cluster
// regards ArgoCD integrations
// nolint:lll
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegisterSpec defines the desired state of Register
type RegisterSpec struct {
	// ArgoCDEndpoint is the endpoint used to
	ArgoCDEndpoint string `json:"argoCDEndpoint"`
}

// RegisterStatus defines the observed state of Register
type RegisterStatus struct {

	// Represents the observations of a Register's current state.
	// Register.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// Register.status.conditions.status are one of True, False, Unknown.
	// Register.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Register is the Schema for the registers API
type Register struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegisterSpec   `json:"spec,omitempty"`
	Status RegisterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RegisterList contains a list of Register
type RegisterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Register `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Register{}, &RegisterList{})
}
