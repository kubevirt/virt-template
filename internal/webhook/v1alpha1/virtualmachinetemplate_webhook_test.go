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

package v1alpha1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	webhookv1alpha1 "kubevirt.io/virt-template/internal/webhook/v1alpha1"
)

var _ = Describe("VirtualMachineTemplate Webhook", func() {
	const (
		param1Name = "NAME"
		param2Name = "PREFERENCE"
		param3Name = "COUNT"

		invalidVMWithoutParam = `{"metadata":{"something":"something"}}`
		invalidVMWithParam    = `{"metadata":{"something":"${NAME}"}}`
	)

	var validator webhookv1alpha1.VirtualMachineTemplateCustomValidator

	BeforeEach(func() {
		validator = webhookv1alpha1.VirtualMachineTemplateCustomValidator{}
	})

	validateOnCreate := func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error) {
		return validator.ValidateCreate(context.Background(), tpl)
	}

	validateOnUpdate := func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error) {
		return validator.ValidateUpdate(context.Background(), nil, tpl)
	}

	DescribeTable("should accept a template with all parameters referenced",
		func(validate func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error)) {
			tpl := newVirtualMachineTemplateWithSpec(
				&v1alpha1.VirtualMachineTemplateSpec{
					Parameters: []v1alpha1.Parameter{
						{
							Name: param1Name,
						},
						{
							Name: param2Name,
						},
					},
					VirtualMachine: &runtime.RawExtension{
						Raw: []byte(`{"metadata":{"name":"${NAME}"},"spec":{"preference":{"name":"${PREFERENCE}"}}}`),
					},
				},
			)

			warnings, err := validate(tpl)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		},
		Entry("on create", validateOnCreate),
		Entry("on update", validateOnUpdate),
	)

	DescribeTable("should reject a template with undefined parameter reference",
		func(validate func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error)) {
			tpl := newVirtualMachineTemplateWithSpec(
				&v1alpha1.VirtualMachineTemplateSpec{
					Parameters: []v1alpha1.Parameter{
						{
							Name: param1Name,
						},
					},
					VirtualMachine: &runtime.RawExtension{
						Raw: []byte(`{"metadata":{"name":"${NAME}"},"spec":{"preference":{"name":"${PREFERENCE}"}}}`),
					},
				},
			)

			warnings, err := validate(tpl)
			Expect(err).To(MatchError(ContainSubstring("references undefined parameter PREFERENCE")))
			Expect(warnings).To(BeEmpty())
		},
		Entry("on create", validateOnCreate),
		Entry("on update", validateOnUpdate),
	)

	DescribeTable("should warn about unused parameter",
		func(validate func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error)) {
			tpl := newVirtualMachineTemplateWithSpec(
				&v1alpha1.VirtualMachineTemplateSpec{
					Parameters: []v1alpha1.Parameter{
						{
							Name: param1Name,
						},
						{
							Name: param2Name,
						},
					},
					VirtualMachine: &runtime.RawExtension{
						Raw: []byte(`{"metadata":{"name":"${NAME}"}}`),
					},
				},
			)

			warnings, err := validate(tpl)
			Expect(err).ToNot(HaveOccurred())
			Expect(warnings).To(ConsistOf(ContainSubstring("PREFERENCE is defined but never referenced")))
		},
		Entry("on create", validateOnCreate),
		Entry("on update", validateOnUpdate),
	)

	DescribeTable("should reject and warn for template with both undefined and unused parameters",
		func(validate func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error)) {
			tpl := newVirtualMachineTemplateWithSpec(
				&v1alpha1.VirtualMachineTemplateSpec{
					Parameters: []v1alpha1.Parameter{
						{
							Name: param1Name,
						},
						{
							Name: param3Name,
						},
					},
					VirtualMachine: &runtime.RawExtension{
						Raw: []byte(`{"metadata":{"name":"${NAME}"},"spec":{"preference":{"name":"${PREFERENCE}"}}}`),
					},
				},
			)

			warnings, err := validate(tpl)
			Expect(err).To(MatchError(ContainSubstring("references undefined parameter PREFERENCE")))
			Expect(warnings).To(ConsistOf(ContainSubstring("COUNT is defined but never referenced")))
		},
		Entry("on create", validateOnCreate),
		Entry("on update", validateOnUpdate),
	)

	DescribeTable("should skip processing validation and warn when required parameter has no value or generator",
		func(validate func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error), params []v1alpha1.Parameter) {
			tpl := newVirtualMachineTemplateWithSpec(
				&v1alpha1.VirtualMachineTemplateSpec{
					Parameters:     params,
					VirtualMachine: &runtime.RawExtension{Raw: []byte(invalidVMWithParam)},
				},
			)

			warnings, err := validate(tpl)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(warnings)).To(BeNumerically(">=", len(params)))
			for _, param := range params {
				Expect(warnings).To(ContainElement(And(ContainSubstring("processing validation skipped"), ContainSubstring(param.Name))))
			}
		},
		Entry("single param on create", validateOnCreate,
			[]v1alpha1.Parameter{{Name: param1Name, Required: true}}),
		Entry("single param on update", validateOnUpdate,
			[]v1alpha1.Parameter{{Name: param1Name, Required: true}}),
		Entry("multiple params on create", validateOnCreate,
			[]v1alpha1.Parameter{{Name: param1Name, Required: true}, {Name: param2Name, Required: true}}),
		Entry("multiple params on update", validateOnUpdate,
			[]v1alpha1.Parameter{{Name: param1Name, Required: true}, {Name: param2Name, Required: true}}),
	)

	DescribeTable("should reject invalid template when processing validation runs",
		func(validate func(tpl *v1alpha1.VirtualMachineTemplate) (admission.Warnings, error), raw string, params []v1alpha1.Parameter) {
			tpl := newVirtualMachineTemplateWithSpec(
				&v1alpha1.VirtualMachineTemplateSpec{
					Parameters: params,
					VirtualMachine: &runtime.RawExtension{
						Raw: []byte(raw),
					},
				},
			)

			warnings, err := validate(tpl)
			Expect(err).To(MatchError(ContainSubstring("processing validation failed")))
			Expect(warnings).To(BeEmpty())
		},
		Entry("no params on create", validateOnCreate, invalidVMWithoutParam, nil),
		Entry("no params on update", validateOnUpdate, invalidVMWithoutParam, nil),
		Entry("required param with value on create", validateOnCreate, invalidVMWithParam, []v1alpha1.Parameter{
			{Name: param1Name, Required: true, Value: "test-vm"},
		}),
		Entry("required param with value on update", validateOnUpdate, invalidVMWithParam, []v1alpha1.Parameter{
			{Name: param1Name, Required: true, Value: "test-vm"},
		}),
		Entry("required param with generator on create", validateOnCreate, invalidVMWithParam, []v1alpha1.Parameter{
			{Name: param1Name, Required: true, Generate: "expression", From: "[a-z]{8}"},
		}),
		Entry("required param with generator on update", validateOnUpdate, invalidVMWithParam, []v1alpha1.Parameter{
			{Name: param1Name, Required: true, Generate: "expression", From: "[a-z]{8}"},
		}),
	)
})

func newVirtualMachineTemplateWithSpec(spec *v1alpha1.VirtualMachineTemplateSpec) *v1alpha1.VirtualMachineTemplate {
	return &v1alpha1.VirtualMachineTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       "VirtualMachineTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: *spec,
	}
}
