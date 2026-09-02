/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, softwarec
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 *
 */

package tests_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template-api/core/subresourcesv1beta1"
	"kubevirt.io/virt-template-api/core/v1beta1"
)

const (
	cpuCountParam = "CPU_COUNT"
	vmSpecKey     = "spec"
)

func vmCPUCountTemplateUnstructured() map[string]any {
	return map[string]any{
		"apiVersion": "kubevirt.io/v1",
		"kind":       "VirtualMachine",
		vmSpecKey: map[string]any{
			"template": map[string]any{
				vmSpecKey: map[string]any{
					"domain": map[string]any{
						"cpu": map[string]any{
							"cores": "${{" + cpuCountParam + "}}",
						},
					},
				},
			},
		},
	}
}

//nolint:dupl
var _ = Describe("VirtualMachineTemplate", Ordered, func() {
	It("should process a VirtualMachineTemplate and return a VirtualMachine", func() {
		const desiredCPUs = 4

		template := &v1beta1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "simple-template",
				Namespace: NamespaceTest,
			},
			Spec: v1beta1.VirtualMachineTemplateSpec{
				Parameters: []v1beta1.Parameter{
					{
						Name:  cpuCountParam,
						Value: "2",
					},
				},
				VirtualMachine: &runtime.RawExtension{
					Object: &unstructured.Unstructured{
						Object: vmCPUCountTemplateUnstructured(),
					},
				},
			},
		}
		_, err := tplClient.TemplateV1beta1().VirtualMachineTemplates(NamespaceTest).
			Create(context.Background(), template, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Process with parameter override
		opts := subresourcesv1beta1.ProcessOptions{
			Parameters: map[string]string{cpuCountParam: fmt.Sprintf("%d", desiredCPUs)},
		}
		processed, err := tplClient.TemplateV1beta1().VirtualMachineTemplates(NamespaceTest).Process(context.Background(), template.Name, opts)
		Expect(err).NotTo(HaveOccurred())
		Expect(processed.VirtualMachine).NotTo(BeNil())
		Expect(processed.VirtualMachine.Spec.Template.Spec.Domain.CPU.Cores).To(Equal(uint32(desiredCPUs)))
	})

	It("should create a VirtualMachine from a VirtualMachineTemplate using a common instance type", func() {
		template := &v1beta1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "create-template",
				Namespace: NamespaceTest,
			},
			Spec: v1beta1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vm-from-template",
						},
						Spec: virtv1.VirtualMachineSpec{
							RunStrategy: ptr.To(virtv1.RunStrategyHalted),
							Instancetype: &virtv1.InstancetypeMatcher{
								Name: testInstancetype,
							},
							Preference: &virtv1.PreferenceMatcher{
								Name: testPreference,
							},
							Template: &virtv1.VirtualMachineInstanceTemplateSpec{},
						},
					},
				},
			},
		}
		createdTemplate, err := tplClient.TemplateV1beta1().VirtualMachineTemplates(NamespaceTest).
			Create(context.Background(), template, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// The template should be placed in the expected namespace.
		Expect(createdTemplate.Namespace).To(Equal(NamespaceTest))

		// The stored template should reference the selected common instance type and preference correctly.
		templateVM := decodeFunctestVM(createdTemplate.Spec.VirtualMachine.Raw)
		Expect(templateVM.Spec.Instancetype).NotTo(BeNil())
		Expect(templateVM.Spec.Instancetype.Name).To(Equal(testInstancetype))
		Expect(templateVM.Spec.Preference).NotTo(BeNil())
		Expect(templateVM.Spec.Preference.Name).To(Equal(testPreference))

		// Create VM from template
		opts := subresourcesv1beta1.ProcessOptions{}
		processed, err := tplClient.TemplateV1beta1().VirtualMachineTemplates(NamespaceTest).CreateVirtualMachine(
			context.Background(),
			createdTemplate.Name,
			opts,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(processed).NotTo(BeNil())
		Expect(processed.VirtualMachine.RunStrategy()).To(Equal(virtv1.RunStrategyHalted))
		Expect(processed.VirtualMachine.Spec.Instancetype).NotTo(BeNil())
		Expect(processed.VirtualMachine.Spec.Instancetype.Name).To(Equal(testInstancetype))
		Expect(processed.VirtualMachine.Spec.Preference).NotTo(BeNil())
		Expect(processed.VirtualMachine.Spec.Preference.Name).To(Equal(testPreference))

		// The created VM should exist in the expected namespace and keep the instance type and preference references.
		createdVM, err := virtClient.VirtualMachine(NamespaceTest).Get(context.Background(), processed.VirtualMachine.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(createdVM.Namespace).To(Equal(NamespaceTest))
		Expect(createdVM.Spec.Instancetype).NotTo(BeNil())
		Expect(createdVM.Spec.Instancetype.Name).To(Equal(testInstancetype))
		Expect(createdVM.Spec.Preference).NotTo(BeNil())
		Expect(createdVM.Spec.Preference.Name).To(Equal(testPreference))

		// Clean up created VM
		err = virtClient.VirtualMachine(NamespaceTest).Delete(context.Background(), processed.VirtualMachine.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})
