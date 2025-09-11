/*
Copyright 2025.

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

package subresourcesv1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	virtv1 "kubevirt.io/api/core/v1"
)

// +kubebuilder:object:root=true

// VirtualMachineTemplate is a dummy object to satisfy the k8s.io/apiserver conventions.
// A subresource cannot be served without a storage for its parent resource.
type VirtualMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ProcessOptions are the options used when processing a VirtualMachineTemplate.
type ProcessOptions struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	Foo string `json:"foo,omitempty"`
}

// +kubebuilder:object:root=true

// ProcessedVirtualMachineTemplate is the object served by the /process subresource.
// It's not a standalone resource but represents a process action on the parent VirtualMachineTemplate resource.
type ProcessedVirtualMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	VirtualMachine *virtv1.VirtualMachine `json:"virtualMachine,omitempty"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineTemplate{}, &ProcessOptions{}, &ProcessedVirtualMachineTemplate{})
}
