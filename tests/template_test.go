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

	subresourcesv1alpha1 "kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	templatev1alpha1 "kubevirt.io/virt-template-api/core/v1alpha1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
)

var _ = Describe("VirtualMachineTemplate", Ordered, func() {
	var client templateclient.Interface

	BeforeAll(func() {
		var err error

		client, err = templateclient.NewForConfig(virtClient.Config())
		Expect(err).NotTo(HaveOccurred())
	})

	It("should process a VirtualMachineTemplate and return a VirtualMachine", func() {
		const desiredCPUs = 4

		template := &templatev1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "simple-template",
				Namespace: NamespaceTest,
			},
			Spec: templatev1alpha1.VirtualMachineTemplateSpec{
				Parameters: []templatev1alpha1.Parameter{
					{
						Name:  "CPU_COUNT",
						Value: "2",
					},
				},
				VirtualMachine: &runtime.RawExtension{
					Object: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "kubevirt.io/v1",
							"kind":       "VirtualMachine",
							"spec": map[string]any{
								"template": map[string]any{
									"spec": map[string]any{
										"domain": map[string]any{
											"cpu": map[string]any{
												"cores": "${{CPU_COUNT}}",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		_, err := client.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).Create(context.Background(), template, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Process with parameter override
		opts := subresourcesv1alpha1.ProcessOptions{
			Parameters: map[string]string{"CPU_COUNT": fmt.Sprintf("%d", desiredCPUs)},
		}
		processed, err := client.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).Process(context.Background(), template.Name, opts)
		Expect(err).NotTo(HaveOccurred())
		Expect(processed.VirtualMachine).NotTo(BeNil())
		Expect(processed.VirtualMachine.Spec.Template.Spec.Domain.CPU.Cores).To(Equal(uint32(desiredCPUs)))
	})

	It("should create a VirtualMachine from VirtualMachineTemplate", func() {
		template := &templatev1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "create-template",
				Namespace: NamespaceTest,
			},
			Spec: templatev1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vm-from-template",
						},
						Spec: virtv1.VirtualMachineSpec{
							RunStrategy: ptr.To(virtv1.RunStrategyHalted),
							Template:    &virtv1.VirtualMachineInstanceTemplateSpec{},
						},
					},
				},
			},
		}
		_, err := client.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).Create(context.Background(), template, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		// Create VM from template
		opts := subresourcesv1alpha1.ProcessOptions{}
		processed, err := client.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).CreateVirtualMachine(
			context.Background(),
			template.Name,
			opts)
		Expect(err).NotTo(HaveOccurred())
		Expect(processed).NotTo(BeNil())
		Expect(processed.VirtualMachine.RunStrategy()).To(Equal(virtv1.RunStrategyHalted))

		// Clean up created VM
		err = virtClient.VirtualMachine(NamespaceTest).Delete(context.Background(), processed.VirtualMachine.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
})
