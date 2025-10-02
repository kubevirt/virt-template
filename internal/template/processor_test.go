package template_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template/api/v1alpha1"
	"kubevirt.io/virt-template/internal/template"
)

var _ = Describe("Processor", func() {
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
		param4Val         = "cores"
		param5Name        = "ISOLATE_EMULATOR_THREAD"
		param5Val         = "true"
	)

	type processor interface {
		Process(tpl *v1alpha1.VirtualMachineTemplate) (*virtv1.VirtualMachine, string, error)
	}

	var p processor

	BeforeEach(func() {
		p = template.NewDefaultProcessor()
	})

	It("should return error for parameter generation failure", func() {
		tmpl := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:     param1Name,
						Generate: "unknown",
					},
				},
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{},
				},
			},
		}

		vm, msg, err := p.Process(tmpl)
		Expect(err).To(MatchError(ContainSubstring("spec.parameters[0].generate: Invalid value: \"unknown\"")))
		Expect(vm).To(BeNil())
		Expect(msg).To(BeEmpty())
	})

	DescribeTable("should return error when trying to process empty virtualMachine", func(templateVM *runtime.RawExtension) {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: templateVM,
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).To(MatchError("spec.virtualMachine: Invalid value: null: virtualMachine is required and cannot be empty"))
		Expect(vm).To(BeNil())
		Expect(msg).To(BeEmpty())
	},
		Entry("nil", nil),
		Entry("empty object", &runtime.RawExtension{}),
	)

	It("should return error for invalid raw VM data", func() {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Raw: []byte(`invalid json`),
				},
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).To(MatchError(ContainSubstring("spec.virtualMachine.raw: Invalid value")))
		Expect(vm).To(BeNil())
		Expect(msg).To(BeEmpty())
	})

	It("should return error when trying to set string field to non-string value", func() {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:  param3Name,
						Value: param3Val,
					},
				},
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{
						ObjectMeta: metav1.ObjectMeta{
							Name: param3Placeholder,
						},
					},
				},
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).To(MatchError(ContainSubstring("attempted to set String field to non-string value '5'")))
		Expect(vm).To(BeNil())
		Expect(msg).To(BeEmpty())
	})

	It("should return error when trying to process non-VM object", func() {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:  param1Name,
						Value: param1Val,
					},
				},
				VirtualMachine: &runtime.RawExtension{
					Object: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: param1Placeholder,
						},
					},
				},
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).To(MatchError("unable to convert into VirtualMachine: object is *v1.Pod"))
		Expect(vm).To(BeNil())
		Expect(msg).To(BeEmpty())
	})

	DescribeTable("should return error when encountering unknown field with", func(templateVM *runtime.RawExtension) {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:  param1Name,
						Value: param1Val,
					},
				},
				VirtualMachine: templateVM,
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).To(MatchError(ContainSubstring("strict decoding error: unknown field \"spec.somefield\"")))
		Expect(vm).To(BeNil())
		Expect(msg).To(BeEmpty())
	},
		Entry("VM in Raw field", &runtime.RawExtension{
			Raw: []byte(`{
                  "apiVersion": "kubevirt.io/v1",
                  "kind": "VirtualMachine",
                  "metadata": {
                    "name": "${NAME}"
                  },
                  "spec": {
                    "somefield": {}
                  }
			    }`),
		}),
		Entry("unstructured VM in Object field", &runtime.RawExtension{
			Object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachine",
					"metadata": map[string]any{
						"name": param1Placeholder,
					},
					"spec": map[string]any{
						"somefield": map[string]any{},
					},
				},
			},
		}),
	)

	DescribeTable("should process template with", func(templateVM *runtime.RawExtension) {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:  param1Name,
						Value: param1Val,
					},
					{
						Name:  param2Name,
						Value: param2Val,
					},
					{
						Name:  param3Name,
						Value: param3Val,
					},
					{
						Name:  param4Name,
						Value: param4Val,
					},
					{
						Name:  param5Name,
						Value: param5Val,
					},
				},
				VirtualMachine: templateVM,
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).ToNot(HaveOccurred())
		Expect(vm.GetObjectKind().GroupVersionKind()).To(Equal(virtv1.VirtualMachineGroupVersionKind))
		Expect(vm.Spec.Preference.Name).To(Equal(param2Val))
		Expect(vm.Name).To(Equal(param1Val))
		Expect(vm.Spec.Template.Spec.Domain.CPU.Cores).To(Equal(uint32(5)))
		Expect(vm.Spec.Template.Spec.Domain.CPU.IsolateEmulatorThread).To(BeTrue())
		Expect(msg).To(BeEmpty())
	},
		Entry("VM in Raw field", &runtime.RawExtension{
			Raw: []byte(`{
                  "apiVersion": "kubevirt.io/v1",
                  "kind": "VirtualMachine",
                  "metadata": {
                    "name": "${NAME}"
                  },
                  "spec": {
					"preference": {
					  "name": "${PREFERENCE}"
					},
                    "template": {
                      "spec": {
                        "domain": {
                          "cpu": {
                            "${CPU_TYPE}": "${{COUNT}}",
                            "isolateEmulatorThread": "${{ISOLATE_EMULATOR_THREAD}}"
                          }
                        }
                      }
                    }
                  }
			    }`),
		}),
		Entry("unstructured VM in Object field", &runtime.RawExtension{
			Object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "kubevirt.io/v1",
					"kind":       "VirtualMachine",
					"metadata": map[string]any{
						"name": param1Placeholder,
					},
					"spec": map[string]any{
						"preference": map[string]any{
							"name": param2Placeholder,
						},
						"template": map[string]any{
							"spec": map[string]any{
								"domain": map[string]any{
									"cpu": map[string]any{
										"${CPU_TYPE}":           "${{COUNT}}",
										"isolateEmulatorThread": "${{ISOLATE_EMULATOR_THREAD}}",
									},
								},
							},
						},
					},
				},
			},
		}),
		Entry("VM in Object field", &runtime.RawExtension{
			Object: &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: param1Placeholder,
				},
				Spec: virtv1.VirtualMachineSpec{
					Preference: &virtv1.PreferenceMatcher{
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
			},
		}),
	)

	DescribeTable("should process template and force correct GVK with", func(templateVM *runtime.RawExtension) {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:  param1Name,
						Value: param1Val,
					},
				},
				VirtualMachine: templateVM,
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).ToNot(HaveOccurred())
		Expect(vm.GetObjectKind().GroupVersionKind()).To(Equal(virtv1.VirtualMachineGroupVersionKind))
		Expect(vm.Name).To(Equal(param1Val))
		Expect(msg).To(BeEmpty())
	},
		Entry("VM in Raw field", &runtime.RawExtension{
			Raw: []byte(`{
                  "apiVersion": "greatapi.io/v1alpha1",
                  "kind": "SomethingSomething",
                  "metadata": {
                    "name": "${NAME}"
                  }
			    }`),
		}),
		Entry("unstructured VM in Object field", &runtime.RawExtension{
			Object: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "greatapi.io/v1alpha1",
					"kind":       "SomethingSomething",
					"metadata": map[string]any{
						"name": param1Placeholder,
					},
				},
			},
		}),
		Entry("VM in Object field", &runtime.RawExtension{
			Object: &virtv1.VirtualMachine{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "greatapi.io/v1alpha1",
					Kind:       "SomethingSomething",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: param1Placeholder,
				},
			},
		}),
	)

	It("should remove hardcoded namespace", func() {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{
						ObjectMeta: metav1.ObjectMeta{
							Name:      param1Val,
							Namespace: "default",
						},
					},
				},
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).ToNot(HaveOccurred())
		Expect(vm.Name).To(Equal(param1Val))
		Expect(vm.Namespace).To(BeEmpty())
		Expect(msg).To(BeEmpty())
	})

	It("should substitute message field", func() {
		t := &v1alpha1.VirtualMachineTemplate{
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				Parameters: []v1alpha1.Parameter{
					{
						Name:  param1Name,
						Value: param1Val,
					},
				},
				Message: "Created VM: " + param1Placeholder,
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{
						ObjectMeta: metav1.ObjectMeta{
							Name: param1Placeholder,
						},
					},
				},
			},
		}

		vm, msg, err := p.Process(t)
		Expect(err).ToNot(HaveOccurred())
		Expect(vm).ToNot(BeNil())
		Expect(msg).To(Equal("Created VM: " + param1Val))
	})
})
