/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha2

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
)

const (
	// MachineFinalizer allows ReconcileBareMetalMachine to clean up resources associated with AWSMachine before
	// removing it from the apiserver.
	MachineFinalizer = "baremetalmachine.infrastructure.cluster.x-k8s.io"
)

// BareMetalMachineSpec defines the desired state of BareMetalMachine
type BareMetalMachineSpec struct {
	// ProviderID will be the baremetal machine in ProviderID format
	// (baremetal:////<machinename>)
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Image is the image to be provisioned.
	Image Image `json:"image"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the Machine's
	// namespace if not specified.
	// +optional
	UserData *UserDataInput `json:"userData,omitempty"`

	// HostSelector specifies matching criteria for labels on BareMetalHosts.
	// This is used to limit the set of BareMetalHost objects considered for
	// claiming for a Machine.
	// +optional
	HostSelector HostSelector `json:"hostSelector,omitempty"`
}

// IsValid returns an error if the object is not valid, otherwise nil. The
// string representation of the error is suitable for human consumption.
func (s *BareMetalMachineSpec) IsValid() error {
	missing := []string{}
	if s.Image.URL == "" {
		missing = append(missing, "Image.URL")
	}
	if s.Image.Checksum == "" {
		missing = append(missing, "Image.Checksum")
	}
	if s.UserData != nil {
		if s.UserData.Type == "" {
			missing = append(missing, "UserData.Type")
		}
		if s.UserData.UserDataAppend != nil {
			if s.UserData.UserDataAppend.Name == "" {
				missing = append(missing, "UserData.UserDataAppend.Name")
			}
		}
		if s.UserData.UserDataPrepend != nil {
			if s.UserData.UserDataPrepend.Name == "" {
				missing = append(missing, "UserData.UserDataPrepend.Name")
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("Missing fields from ProviderSpec: %v", missing)
	}
	return nil
}

// BareMetalMachineStatus defines the observed state of BareMetalMachine
type BareMetalMachineStatus struct {
	capi.MachineStatus `json:",inline"`

	// Ready is the state of the metal3.
	// TODO : Document the variable :
	// mhrivnak: " it would be good to document what this means, how to interpret
	// it, under what circumstances the value changes, etc."
	// +optional
	Ready bool `json:"ready"`

	// UserData references the Secret that holds user data needed by the bare metal
	// operator. The Namespace is optional; it will default to the Machine's
	// namespace if not specified.
	// +optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`
}

// UserDataInput contains the userdata given by the user as a secret and
// the type and strategy of merge.
type UserDataInput struct {
	//Type is the type of userdata
	// +kubebuilder:validation:Enum=cloud-init
	Type string `json:"type"`

	// UserDataAppend references the Secret that holds user data that will be
	// appended to the CABPK output. The Namespace is optional; it will default to
	// the Machine's namespace if not specified.
	// +optional
	UserDataAppend *corev1.SecretReference `json:"userDataAppend,omitempty"`

	// UserDataPrepend references the Secret that holds user data that will be
	// prepended to the CABPK output. The Namespace is optional; it will default to
	// the Machine's namespace if not specified.
	// +optional
	UserDataPrepend *corev1.SecretReference `json:"userDataPrepend,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=baremetalmachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine is Ready"

// BareMetalMachine is the Schema for the baremetalmachines API
type BareMetalMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BareMetalMachineSpec   `json:"spec,omitempty"`
	Status BareMetalMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BareMetalMachineList contains a list of BareMetalMachine
type BareMetalMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BareMetalMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BareMetalMachine{}, &BareMetalMachineList{})
}
