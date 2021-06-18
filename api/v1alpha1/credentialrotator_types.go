/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CredentialRotatorSpec defines the desired state of CredentialRotator
type CredentialRotatorSpec struct {
	UserAPIKey   string `json:"userAPIKey,omitempty"`
	ServiceGUID  string `json:"serviceGUID,omitempty"`
	ServiceURL   string `json:"serviceURL,omitempty"`
	AppName      string `json:"appName,omitempty"`
	AppNameSpace string `json:"appNameSpace,omitempty"`
}

// CredentialRotatorStatus defines the observed state of CredentialRotator
type CredentialRotatorStatus struct {
	PreviousResourceKeyID string `json:"previousResourceKeyID,omitempty"`
	Phase                 string `json:"phase,omitempty"`
}

const (
	PhasePending   = "PENDING"
	PhaseCreating  = "CREATING"
	PhaseNotifying = "NOTIFYING"
	PhaseDeleting  = "DELETING"
	PhaseDone      = "DONE"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CredentialRotator is the Schema for the credentialrotators API
type CredentialRotator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CredentialRotatorSpec   `json:"spec,omitempty"`
	Status CredentialRotatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CredentialRotatorList contains a list of CredentialRotator
type CredentialRotatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CredentialRotator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CredentialRotator{}, &CredentialRotatorList{})
}
