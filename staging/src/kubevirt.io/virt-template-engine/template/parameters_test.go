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

package template_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/virt-template-api/core/v1alpha1"
	"kubevirt.io/virt-template-engine/template"
)

var _ = Describe("Parameters", func() {
	const (
		param1Name       = "NAME"
		param1DefaultVal = "default-name"
		param1Val        = "test-vm"
		param2Name       = "PREFERENCE"
		param2DefaultVal = "default-preference"
		param2Val        = "fedora"
		param3Name       = "RUNNING"
		paramUnknownName = "UNKNOWN"
		paramUnknownVal  = "something"
	)

	Context("MergeParameters", func() {
		var tplParams []v1alpha1.Parameter

		BeforeEach(func() {
			tplParams = []v1alpha1.Parameter{
				{
					Name:  param1Name,
					Value: param1DefaultVal,
				},
				{
					Name:  param2Name,
					Value: param2DefaultVal,
				},
			}
		})

		It("should merge single parameter successfully", func() {
			params := map[string]string{
				param1Name: param1Val,
			}

			newTplParams, err := template.MergeParameters(tplParams, params)
			Expect(err).ToNot(HaveOccurred())
			Expect(newTplParams[0].Value).To(Equal(param1Val))
			Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
		})

		It("should merge multiple parameters successfully", func() {
			params := map[string]string{
				param1Name: param1Val,
				param2Name: param2Val,
			}

			newTplParams, err := template.MergeParameters(tplParams, params)
			Expect(err).ToNot(HaveOccurred())
			Expect(newTplParams[0].Value).To(Equal(param1Val))
			Expect(newTplParams[1].Value).To(Equal(param2Val))
		})

		It("should handle nil params", func() {
			newTplParams, err := template.MergeParameters(tplParams, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(newTplParams[0].Value).To(Equal(param1DefaultVal))
			Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
		})

		It("should handle empty params", func() {
			newTplParams, err := template.MergeParameters(tplParams, map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			Expect(newTplParams[0].Value).To(Equal(param1DefaultVal))
			Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
		})

		It("should return error for single parameter not in template", func() {
			params := map[string]string{
				paramUnknownName: paramUnknownVal,
			}

			newTplParams, err := template.MergeParameters(tplParams, params)
			Expect(err).To(MatchError(fmt.Sprintf("parameter %s not found in template", paramUnknownName)))
			Expect(newTplParams).To(BeNil())
		})

		It("should return error when one of multiple params not in template", func() {
			params := map[string]string{
				param1Name:       param1Val,
				paramUnknownName: paramUnknownVal,
			}

			newTplParams, err := template.MergeParameters(tplParams, params)
			Expect(err).To(MatchError(fmt.Sprintf("parameter %s not found in template", paramUnknownName)))
			Expect(newTplParams).To(BeNil())
		})

		It("should handle empty template parameters", func() {
			tplParams = []v1alpha1.Parameter{}

			params := map[string]string{
				param1Name: param1Val,
			}

			newTplParams, err := template.MergeParameters(tplParams, params)
			Expect(err).To(MatchError(fmt.Sprintf("parameter %s not found in template", param1Name)))
			Expect(newTplParams).To(BeNil())
		})

		It("should handle parameter with empty value", func() {
			params := map[string]string{
				param1Name: "",
			}

			newTplParams, err := template.MergeParameters(tplParams, params)
			Expect(err).ToNot(HaveOccurred())
			Expect(newTplParams[0].Value).To(BeEmpty())
			Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
		})
	})

	Context("ValidateParameterReferences", func() {
		Context("with valid templates", func() {
			It("should accept a template with all parameters referenced", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
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
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept a template with parameter referenced in message", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
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
						Message: "A good preference for this VM could be ${PREFERENCE}.",
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept a template with non-string parameter syntax ${{KEY}}", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						Parameters: []v1alpha1.Parameter{
							{
								Name: param3Name,
							},
						},
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"spec":{"running":"${{RUNNING}}"}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept a template with no parameters and no references", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"metadata":{"name":"static-name"}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept multiple references to the same parameter", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						Parameters: []v1alpha1.Parameter{
							{
								Name: param1Name,
							},
						},
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"metadata":{"name":"${NAME}","labels":{"app":"${NAME}"}}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(BeEmpty())
			})

			It("should accept parameters with mixed ${} and ${{}} references", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						Parameters: []v1alpha1.Parameter{
							{
								Name: param1Name,
							},
							{
								Name: param3Name,
							},
						},
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"metadata":{"name":"${NAME}"},"spec":{"running":"${{RUNNING}}"}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("with invalid templates", func() {
			It("should reject a template with undefined parameter reference", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						Parameters: []v1alpha1.Parameter{
							{
								Name: param1Name,
							},
						},
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"metadata":{"name":"${NAME}"},"spec":{"preference":{"name":"${PREFERENCE}"}}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("references undefined parameter PREFERENCE"))))
				Expect(warnings).To(BeEmpty())
			})

			It("should warn about unused parameter", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
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
						Message: "VM created successfully",
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(BeEmpty())
				Expect(warnings).To(ConsistOf(ContainSubstring("PREFERENCE is defined but never referenced")))
			})

			It("should reject and warn for template with both undefined and unused parameters", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						Parameters: []v1alpha1.Parameter{
							{
								Name: param3Name,
							},
						},
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"metadata":{"name":"${NAME}"}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("references undefined parameter NAME"))))
				Expect(warnings).To(ConsistOf(ContainSubstring("RUNNING is defined but never referenced")))
			})

			It("should reject undefined parameter in message", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						Parameters: []v1alpha1.Parameter{
							{
								Name: param1Name,
							},
						},
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"metadata":{"name":"${NAME}"}}`),
						},
						Message: "A good preference for this VM could be ${PREFERENCE}.",
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("references undefined parameter PREFERENCE"))))
				Expect(warnings).To(BeEmpty())
			})

			It("should reject undefined non-string parameter reference", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`{"spec":{"running":"${{RUNNING}}"}}`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("references undefined parameter RUNNING"))))
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("with edge cases", func() {
			It("should handle nil virtualMachine", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("virtualMachine is required and cannot be empty"))))
				Expect(warnings).To(BeEmpty())
			})

			It("should handle empty virtualMachine", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						VirtualMachine: &runtime.RawExtension{},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("virtualMachine is required and cannot be empty"))))
				Expect(warnings).To(BeEmpty())
			})

			It("should handle invalid raw extension", func() {
				t := &v1alpha1.VirtualMachineTemplate{
					Spec: v1alpha1.VirtualMachineTemplateSpec{
						VirtualMachine: &runtime.RawExtension{
							Raw: []byte(`gibberish`),
						},
					},
				}

				warnings, errs := template.ValidateParameterReferences(t)
				Expect(errs).To(ConsistOf(MatchError(ContainSubstring("error decoding virtualMachine"))))
				Expect(warnings).To(BeEmpty())
			})
		})
	})
})
