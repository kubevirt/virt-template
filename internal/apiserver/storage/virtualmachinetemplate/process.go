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

package virtualmachinetemplate

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/warning"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template-api/core/subresourcesv1beta1"
	"kubevirt.io/virt-template-api/core/v1beta1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
	"kubevirt.io/virt-template-engine/template"
)

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/status,verbs=get
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=create

const (
	// JSONBufferSize determines how far into the stream the decoder will look for JSON.
	JSONBufferSize = 1024
	// DebugLogLevel is the klog verbosity level for debug messages.
	DebugLogLevel = 5
)

// Processor processes a VirtualMachineTemplate into a VirtualMachine.
type Processor interface {
	Process(tpl *v1beta1.VirtualMachineTemplate) (*virtv1.VirtualMachine, string, *field.Error)
}

// ProcessTemplate fetches the named template, merges parameters from the
// request body, and returns a ProcessedVirtualMachineTemplate.
func ProcessTemplate(
	ctx context.Context,
	client templateclient.Interface,
	processor Processor,
	body io.Reader,
	ns string,
	id string,
) (*subresourcesv1beta1.ProcessedVirtualMachineTemplate, error) {
	opts := &subresourcesv1beta1.ProcessOptions{}
	if err := yaml.NewYAMLOrJSONDecoder(body, JSONBufferSize).Decode(opts); err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("error parsing ProcessOptions: %v", err))
	}

	tpl, err := client.TemplateV1beta1().VirtualMachineTemplates(ns).Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting VirtualMachineTemplate: %w", err))
	}

	tpl.Spec.Parameters, err = template.MergeParameters(tpl.Spec.Parameters, opts.Parameters)
	if err != nil {
		return nil, apierrors.NewConflict(schema.GroupResource{
			Group:    tpl.GroupVersionKind().Group,
			Resource: tpl.Kind,
		}, id, err)
	}

	warnings, errs := template.ValidateParameterReferences(tpl)
	for _, w := range warnings {
		warning.AddWarning(ctx, "", w)
	}
	if len(errs) > 0 {
		return nil, apierrors.NewInvalid(tpl.GroupVersionKind().GroupKind(), id, errs)
	}

	vm, msg, pErr := processor.Process(tpl)
	if pErr != nil {
		return nil, apierrors.NewInvalid(tpl.GroupVersionKind().GroupKind(), id, field.ErrorList{pErr})
	}

	return &subresourcesv1beta1.ProcessedVirtualMachineTemplate{
		TemplateRef: &corev1.ObjectReference{
			Namespace: ns,
			Name:      id,
		},
		VirtualMachine: vm,
		Message:        msg,
	}, nil
}
