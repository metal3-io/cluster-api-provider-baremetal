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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Metal3MachineTemplateSpec defines the desired state of Metal3MachineTemplate
type Metal3MachineTemplateSpec struct {
	Template Metal3MachineTemplateResource `json:"template"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=metal3machinetemplates,scope=Namespaced,categories=cluster-api

// Metal3MachineTemplate is the Schema for the metal3machinetemplates API
type Metal3MachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec Metal3MachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// Metal3MachineTemplateList contains a list of Metal3MachineTemplate
type Metal3MachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Metal3MachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Metal3MachineTemplate{}, &Metal3MachineTemplateList{})
}

// Metal3MachineTemplateResource describes the data needed to create a Metal3Machine from a template
type Metal3MachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec Metal3MachineSpec `json:"spec"`
}
