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

package template

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/template/generator"
)

var _ = Describe("Template", func() {
	const (
		param1Name        = "NAME"
		param1Placeholder = "${NAME}"
		param1Val         = "test-vm"
		param2Name        = "PREFERENCE"
		param2Placeholder = "${PREFERENCE}"
		param2Val         = "fedora"
		param3Name        = "COUNT"
		param3Placeholder = "${{COUNT}}"
		param3Val         = "5"
		param4Name        = "CPU_TYPE"
		param4Placeholder = "${CPU_TYPE}"
		param4Val         = "cores"
		param5Name        = "ISOLATE_EMULATOR_THREAD"
		param5Placeholder = "${{ISOLATE_EMULATOR_THREAD}}"
		param5Val         = "true"
	)

	Describe("GenerateParameterValues", func() {
		var generators map[string]generator.Generator

		BeforeEach(func() {
			generators = map[string]generator.Generator{
				"expression": &generator.ExpressionValue{},
			}
		})

		It("should generate value for parameter with Generate field", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Generate: "expression",
					From:     "[a-z]{8}",
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).ToNot(HaveOccurred())
			Expect(gen).To(HaveLen(1))
			Expect(gen[param1Name].Value).To(MatchRegexp("^[a-z]{8}$"))
		})

		It("should generate values for multiple parameters with Generate field", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Generate: "expression",
					From:     "[a-z]{8}",
				},
				{
					Name:     param3Name,
					Generate: "expression",
					From:     "[0-9]{1}",
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).ToNot(HaveOccurred())
			Expect(gen).To(HaveLen(2))
			Expect(gen[param1Name].Value).To(MatchRegexp("^[a-z]{8}$"))
			Expect(gen[param3Name].Value).To(MatchRegexp("^[0-9]{1}$"))
		})

		It("should use existing value when provided", func() {
			params := []v1alpha1.Parameter{
				{
					Name:  param1Name,
					Value: param1Val,
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).ToNot(HaveOccurred())
			Expect(gen).To(HaveLen(1))
			Expect(gen[param1Name].Value).To(Equal(param1Val))
		})

		It("should not generate value when both Value and Generate are provided", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Value:    param1Val,
					Generate: "expression",
					From:     "[a-z]{8}",
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).ToNot(HaveOccurred())
			Expect(gen).To(HaveLen(1))
			Expect(gen[param1Name].Value).To(Equal(param1Val))
		})

		It("should ignore malformed pattern in From field", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Generate: "expression",
					From:     "[a-z{8}",
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).ToNot(HaveOccurred())
			Expect(gen).To(HaveLen(1))
			Expect(gen[param1Name].Value).To(Equal(params[0].From))
		})

		It("should handle empty parameters list", func() {
			params := []v1alpha1.Parameter{}

			gen, err := generateParameterValues(params, generators)
			Expect(err).ToNot(HaveOccurred())
			Expect(gen).To(BeEmpty())
		})

		It("should return error for unknown generator", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Generate: "unknown",
					From:     "[a-z]{8}",
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).To(MatchError(ContainSubstring("spec.parameters[0].generate: Invalid value: \"unknown\"")))
			Expect(gen).To(BeNil())
		})

		It("should return error for required parameter without value", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Required: true,
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).To(MatchError(ContainSubstring("spec.parameters[0].value: Required value")))
			Expect(gen).To(BeNil())
		})

		It("should return error for parameter with Generate field but empty From", func() {
			params := []v1alpha1.Parameter{
				{
					Name:     param1Name,
					Generate: "expression",
					From:     "",
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).To(MatchError(
				"spec.parameters[0].from: Invalid value: \"\": from cannot be empty for parameter 'NAME' using generator 'expression'",
			))
			Expect(gen).To(BeNil())
		})

		It("should return error for duplicate parameter names", func() {
			params := []v1alpha1.Parameter{
				{
					Name:  param1Name,
					Value: param1Val,
				},
				{
					Name:  param1Name,
					Value: param1Val,
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).To(MatchError("spec.parameters[1].name: Duplicate value: \"NAME\""))
			Expect(gen).To(BeNil())
		})

		It("should return error for empty parameter name", func() {
			params := []v1alpha1.Parameter{
				{
					Name:  "",
					Value: param1Val,
				},
			}

			gen, err := generateParameterValues(params, generators)
			Expect(err).To(MatchError("spec.parameters[0].name: Invalid value: \"\": parameter name is empty"))
			Expect(gen).To(BeNil())
		})
	})

	Describe("removeHardcodedNamespace", func() {
		DescribeTable("should remove hardcoded namespace", func(obj runtime.Object) {
			err := removeHardcodedNamespace(obj)
			Expect(err).ToNot(HaveOccurred())
			objMeta, err := meta.Accessor(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(objMeta.GetNamespace()).To(BeEmpty())
		},
			Entry("from unstructured VirtualMachine object", &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachine",
					"metadata": map[string]any{
						"namespace": "default",
					},
				},
			}),
			Entry("from VirtualMachine object", &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
			}),
		)

		It("should preserve namespace with parameter", func() {
			const paramNS = "${NAMESPACE}"
			vm := &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: paramNS,
				},
			}

			err := removeHardcodedNamespace(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(vm.Namespace).To(Equal(paramNS))
		})

		It("should not preserve namespace with non-string parameter", func() {
			vm := &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "${{NAMESPACE}}",
				},
			}

			err := removeHardcodedNamespace(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(vm.Namespace).To(BeEmpty())
		})

		It("should handle empty namespace", func() {
			vm := &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "",
				},
			}

			err := removeHardcodedNamespace(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(vm.Namespace).To(BeEmpty())
		})
	})

	Describe("Substitution", func() {
		var params map[string]v1alpha1.Parameter

		BeforeEach(func() {
			params = map[string]v1alpha1.Parameter{
				param1Name: {Name: param1Name, Value: param1Val},
				param2Name: {Name: param2Name, Value: param2Val},
				param3Name: {Name: param3Name, Value: param3Val},
				param4Name: {Name: param4Name, Value: param4Val},
				param5Name: {Name: param5Name, Value: param5Val},
			}
		})

		Describe("substituteParameters", func() {
			DescribeTable("should substitute string parameter", func(s, expected string) {
				val, asString, err := substituteParameters(s, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal(expected))
				Expect(asString).To(BeTrue())
			},
				Entry("only param", param1Placeholder, param1Val),
				Entry("with prefix", "prefix-"+param1Placeholder, "prefix-"+param1Val),
				Entry("with suffix", param1Placeholder+"-suffix", param1Val+"-suffix"),
				Entry("with prefix and suffix", "prefix-"+param1Placeholder+"-suffix", "prefix-"+param1Val+"-suffix"),
			)

			It("should substitute same parameter multiple times", func() {
				val, asString, err := substituteParameters(param1Placeholder+"-"+param1Placeholder, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal(param1Val + "-" + param1Val))
				Expect(asString).To(BeTrue())
			})

			It("should substitute multiple parameters", func() {
				val, asString, err := substituteParameters(param1Placeholder+"-${COUNT}", params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal("test-vm-5"))
				Expect(asString).To(BeTrue())
			})

			It("should substitute non-string parameter", func() {
				val, asString, err := substituteParameters(param3Placeholder, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal(param3Val))
				Expect(asString).To(BeFalse())
			})

			It("should not substitute multiple non-string parameters", func() {
				const in = param3Placeholder + "-" + param3Placeholder
				val, asString, err := substituteParameters(in, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal(in))
				Expect(asString).To(BeTrue())
			})

			It("should not substitute invalid parameter names with hyphens", func() {
				val, asString, err := substituteParameters("${NA-ME}", params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal("${NA-ME}"))
				Expect(asString).To(BeTrue())
			})

			It("should not substitute parameter names with spaces", func() {
				val, asString, err := substituteParameters("${NA ME}", params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal("${NA ME}"))
				Expect(asString).To(BeTrue())
			})

			It("should handle nested parameter-like syntax", func() {
				val, asString, err := substituteParameters("${${NAME}}", params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal("${" + param1Val + "}"))
				Expect(asString).To(BeTrue())
			})

			It("should handle string without parameters", func() {
				val, asString, err := substituteParameters("no-params", params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal("no-params"))
				Expect(asString).To(BeTrue())
			})

			It("should not substitute ${{KEY}} when not exact match", func() {
				const in = "prefix-" + param3Placeholder
				val, asString, err := substituteParameters(in, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(Equal(in))
				Expect(asString).To(BeTrue())
			})

			It("should return error on unknown parameter", func() {
				val, asString, err := substituteParameters("${UNKNOWN}", params)
				Expect(err).To(MatchError("found parameter 'UNKNOWN' but it was not defined"))
				Expect(val).To(BeEmpty())
				Expect(asString).To(BeFalse())
			})

			DescribeTable("should return error when param not found", func(params map[string]v1alpha1.Parameter) {
				val, asString, err := substituteParameters(param1Placeholder, params)
				Expect(err).To(MatchError("found parameter 'NAME' but it was not defined"))
				Expect(val).To(BeEmpty())
				Expect(asString).To(BeFalse())
			},
				Entry("on empty map", make(map[string]v1alpha1.Parameter)),
				Entry("on nil map", nil),
			)

			DescribeTable("should handle params map", func(params map[string]v1alpha1.Parameter) {
				val, asString, err := substituteParameters("", params)
				Expect(err).ToNot(HaveOccurred())
				Expect(val).To(BeEmpty())
				Expect(asString).To(BeTrue())
			},
				Entry("when empty", make(map[string]v1alpha1.Parameter)),
				Entry("when nil", nil),
			)
		})

		Describe("substituteAllParameters", func() {
			It("should substitute parameters in unstructured VM object", func() {
				obj := &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "kubevirt.io/v1",
						"kind":       "VirtualMachine",
						"metadata": map[string]any{
							"name": param1Placeholder,
						},
						"spec": map[string]any{
							"instancetype": map[string]any{
								"name": param2Placeholder,
							},
							"template": map[string]any{
								"spec": map[string]any{
									"domain": map[string]any{
										"cpu": map[string]any{
											param4Placeholder:       param3Placeholder,
											"isolateEmulatorThread": param5Placeholder,
										},
									},
								},
							},
						},
					},
				}

				err := substituteAllParameters(obj, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj.GetName()).To(Equal(param1Val))

				instancetype, found, err := unstructured.NestedString(obj.Object, "spec", "instancetype", "name")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(instancetype).To(Equal(param2Val))

				cpu, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "spec", "domain", "cpu")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(cpu).To(Equal(map[string]any{
					param4Val:               float64(5),
					"isolateEmulatorThread": true,
				}))
			})

			It("should substitute parameters in VM object", func() {
				vm := &virtv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: param1Placeholder,
					},
					Spec: virtv1.VirtualMachineSpec{
						Instancetype: &virtv1.InstancetypeMatcher{
							Name: param2Placeholder,
						},
						Template: &virtv1.VirtualMachineInstanceTemplateSpec{
							Spec: virtv1.VirtualMachineInstanceSpec{
								Domain: virtv1.DomainSpec{
									CPU: &virtv1.CPU{
										// Cannot template non-string fields when using the concrete VirtualMachine type
										Cores:                 uint32(5),
										IsolateEmulatorThread: true,
									},
								},
							},
						},
					},
				}

				err := substituteAllParameters(vm, params)
				Expect(err).ToNot(HaveOccurred())
				Expect(vm.Name).To(Equal(param1Val))
				Expect(vm.Spec.Instancetype.Name).To(Equal(param2Val))
				Expect(vm.Spec.Template.Spec.Domain.CPU.Cores).To(Equal(uint32(5)))
				Expect(vm.Spec.Template.Spec.Domain.CPU.IsolateEmulatorThread).To(BeTrue())
			})
		})
	})

	Describe("collectReferencedParameters", func() {
		It("should extract single ${KEY} parameter", func() {
			params := collectReferencedParameters(param1Placeholder)
			Expect(params).To(HaveLen(1))
			Expect(params).To(HaveKey(param1Name))
		})

		It("should extract single ${{KEY}} parameter", func() {
			params := collectReferencedParameters(param3Placeholder)
			Expect(params).To(HaveLen(1))
			Expect(params).To(HaveKey(param3Name))
		})

		It("should extract multiple ${KEY} parameters", func() {
			params := collectReferencedParameters(param1Placeholder + "-" + param2Placeholder)
			Expect(params).To(HaveLen(2))
			Expect(params).To(HaveKey(param1Name))
			Expect(params).To(HaveKey(param2Name))
		})

		It("should extract same parameter referenced multiple times", func() {
			params := collectReferencedParameters(param1Placeholder + "-" + param1Placeholder)
			Expect(params).To(HaveLen(1))
			Expect(params).To(HaveKey(param1Name))
		})

		It("should handle string with no parameters", func() {
			params := collectReferencedParameters("no-params-here")
			Expect(params).To(BeEmpty())
		})

		It("should handle empty string", func() {
			params := collectReferencedParameters("")
			Expect(params).To(BeEmpty())
		})
	})

	Describe("collectAllReferencedParameters", func() {
		It("should collect parameters from unstructured object", func() {
			obj := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name": param1Placeholder,
					},
					"spec": map[string]any{
						"preference": map[string]any{
							"name": param2Placeholder,
						},
					},
				},
			}

			params, err := collectAllReferencedParameters(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(params).To(HaveLen(2))
			Expect(params).To(HaveKey(param1Name))
			Expect(params).To(HaveKey(param2Name))
		})

		It("should collect parameters from VirtualMachine object", func() {
			vm := &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: param1Placeholder,
				},
				Spec: virtv1.VirtualMachineSpec{
					Preference: &virtv1.PreferenceMatcher{
						Name: param2Placeholder,
					},
				},
			}

			params, err := collectAllReferencedParameters(vm)
			Expect(err).ToNot(HaveOccurred())
			Expect(params).To(HaveLen(2))
			Expect(params).To(HaveKey(param1Name))
			Expect(params).To(HaveKey(param2Name))
		})

		It("should collect both ${} and ${{}} parameters", func() {
			obj := &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachine",
					"metadata": map[string]any{
						"name": param1Placeholder,
					},
					"spec": map[string]any{
						"instancetype": map[string]any{
							"name": param2Placeholder,
						},
						"template": map[string]any{
							"spec": map[string]any{
								"domain": map[string]any{
									"cpu": map[string]any{
										param4Placeholder:       param3Placeholder,
										"isolateEmulatorThread": param5Placeholder,
									},
								},
							},
						},
					},
				},
			}

			params, err := collectAllReferencedParameters(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(params).To(HaveLen(5))
			Expect(params).To(HaveKey(param1Name))
			Expect(params).To(HaveKey(param2Name))
			Expect(params).To(HaveKey(param3Name))
			Expect(params).To(HaveKey(param4Name))
			Expect(params).To(HaveKey(param5Name))
		})

		It("should handle object with no parameters", func() {
			obj := &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"name": "static-name",
					},
				},
			}

			params, err := collectAllReferencedParameters(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(params).To(BeEmpty())
		})
	})
})
