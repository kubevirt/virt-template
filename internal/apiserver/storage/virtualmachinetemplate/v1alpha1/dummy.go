/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	templateapi "kubevirt.io/virt-template-api/core"
	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
)

// V1alpha1DummyREST is required to satisfy the k8s.io/apiserver conventions.
// A subresource cannot be served without a storage for its parent resource.
type V1alpha1DummyREST struct{}

func NewV1alpha1DummyREST() *V1alpha1DummyREST {
	return &V1alpha1DummyREST{}
}

var (
	_ = rest.Storage(&V1alpha1DummyREST{})
	_ = rest.Scoper(&V1alpha1DummyREST{})
	_ = rest.SingularNameProvider(&V1alpha1DummyREST{})
)

func (r *V1alpha1DummyREST) New() runtime.Object {
	return &subresourcesv1alpha1.VirtualMachineTemplate{}
}

func (r *V1alpha1DummyREST) Destroy() {}

func (r *V1alpha1DummyREST) NamespaceScoped() bool { return true }

func (r *V1alpha1DummyREST) GetSingularName() string {
	return templateapi.SingularResourceName
}
