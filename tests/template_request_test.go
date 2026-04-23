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
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	virtv1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	"kubevirt.io/virt-template-api/core/v1alpha1"
)

const (
	sourceMACAddress = "02:00:00:00:00:01"
	sourceSerial     = "cf5d0f13-7075-48fe-a8e6-108f74e13633"
	sourceUUID       = "354b1c05-8b0e-446c-8a0d-43d897f96c25"
)

var _ = Describe("VirtualMachineTemplateRequest", func() {
	It("should create a VirtualMachineTemplate from an existing VirtualMachine", func() {
		vm, err := virtClient.VirtualMachine(NamespaceSecondaryTest).Create(context.Background(), newVM(), metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			vm, err = virtClient.VirtualMachine(NamespaceSecondaryTest).Get(context.Background(), vm.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(vm.Status.Ready).To(BeTrue())
		}, 5*time.Minute, 1*time.Second).Should(Succeed())

		tplReq := &v1alpha1.VirtualMachineTemplateRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "virt-template-",
				Namespace:    NamespaceTest,
			},
			Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
				VirtualMachineRef: v1alpha1.VirtualMachineReference{
					Namespace: vm.Namespace,
					Name:      vm.Name,
				},
			},
		}

		tplReq, err = tplClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
			Create(context.Background(), tplReq, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		tplReq = waitForTemplateRequestReady(tplReq.Name)

		tpl, err := tplClient.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).
			Get(context.Background(), tplReq.Status.TemplateRef.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		tplVM := decodeFunctestVM(tpl.Spec.VirtualMachine.Raw)
		for _, iface := range tplVM.Spec.Template.Spec.Domain.Devices.Interfaces {
			Expect(iface.MacAddress).To(BeEmpty(), "MAC address should be stripped from interface %s", iface.Name)
		}
		if tplVM.Spec.Template.Spec.Domain.Firmware != nil {
			Expect(tplVM.Spec.Template.Spec.Domain.Firmware.Serial).To(BeEmpty(), "firmware serial should be stripped")
			Expect(tplVM.Spec.Template.Spec.Domain.Firmware.UUID).To(BeEmpty(), "firmware UUID should be stripped")
		}

		name := "my-created-vm-" + rand.String(5)
		processedVM, err := tplClient.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).CreateVirtualMachine(
			context.Background(), tpl.Name,
			subresourcesv1alpha1.ProcessOptions{
				Parameters: map[string]string{
					"NAME": name,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(processedVM.VirtualMachine.Name).To(Equal(name))

		var createdVM *virtv1.VirtualMachine
		Eventually(func(g Gomega) {
			createdVM, err = virtClient.VirtualMachine(NamespaceTest).Get(context.Background(), processedVM.VirtualMachine.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(createdVM.Status.Ready).To(BeTrue())
		}, 5*time.Minute, 1*time.Second).Should(Succeed())

		for _, iface := range createdVM.Spec.Template.Spec.Domain.Devices.Interfaces {
			Expect(iface.MacAddress).ToNot(Equal(sourceMACAddress),
				"created VM should not have same MAC address as source VM on interface %s", iface.Name)
		}
		if createdVM.Spec.Template.Spec.Domain.Firmware != nil {
			Expect(createdVM.Spec.Template.Spec.Domain.Firmware.Serial).ToNot(Equal(sourceSerial),
				"created VM should not have same firmware serial as source VM")
			Expect(createdVM.Spec.Template.Spec.Domain.Firmware.UUID).ToNot(BeEquivalentTo(sourceUUID),
				"created VM should not have same firmware UUID as source VM")
		}
	})

	It("should create a VirtualMachineTemplate from a VM with backend storage", func() {
		vm, err := virtClient.VirtualMachine(NamespaceSecondaryTest).Create(
			context.Background(), newVMWithPersistentEFI(), metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			vm, err = virtClient.VirtualMachine(NamespaceSecondaryTest).Get(context.Background(), vm.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(vm.Status.Ready).To(BeTrue())
		}, 5*time.Minute, 1*time.Second).Should(Succeed())

		tplReq := &v1alpha1.VirtualMachineTemplateRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "virt-template-efi-",
				Namespace:    NamespaceTest,
			},
			Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
				VirtualMachineRef: v1alpha1.VirtualMachineReference{
					Namespace: vm.Namespace,
					Name:      vm.Name,
				},
			},
		}

		tplReq, err = tplClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
			Create(context.Background(), tplReq, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		tplReq = waitForTemplateRequestReady(tplReq.Name)

		tpl, err := tplClient.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).
			Get(context.Background(), tplReq.Status.TemplateRef.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		// Verify template does not include DataVolumeTemplates for backend storage
		vmFromTemplate := decodeFunctestVM(tpl.Spec.VirtualMachine.Raw)
		// Should only have DataVolumeTemplate for the data disk, not for backend storage
		Expect(vmFromTemplate.Spec.DataVolumeTemplates).To(HaveLen(1))

		// Create a VM from the template
		name := "my-created-vm-efi-" + rand.String(5)
		processedResult, err := tplClient.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).CreateVirtualMachine(
			context.Background(), tpl.Name,
			subresourcesv1alpha1.ProcessOptions{
				Parameters: map[string]string{
					"NAME": name,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(processedResult.VirtualMachine.Name).To(Equal(name))

		Eventually(func(g Gomega) {
			newVM, err := virtClient.VirtualMachine(NamespaceTest).Get(
				context.Background(), processedResult.VirtualMachine.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(newVM.Status.Ready).To(BeTrue())
		}, 5*time.Minute, 1*time.Second).Should(Succeed())
	})
})

func waitForTemplateRequestReady(name string) *v1alpha1.VirtualMachineTemplateRequest {
	var tplReq *v1alpha1.VirtualMachineTemplateRequest
	Eventually(func(g Gomega) {
		var err error
		tplReq, err = tplClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
			Get(context.Background(), name, metav1.GetOptions{})
		g.Expect(err).ToNot(HaveOccurred())
		cond := meta.FindStatusCondition(tplReq.Status.Conditions, v1alpha1.ConditionReady)
		g.Expect(cond).ToNot(BeNil())
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	}, 5*time.Minute, 1*time.Second).Should(Succeed())
	return tplReq
}

func newVM() *virtv1.VirtualMachine {
	suffix := rand.String(5)
	dvName := "my-test-dv-" + suffix
	return &virtv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-test-vm-" + suffix,
			Namespace: NamespaceSecondaryTest,
		},
		Spec: virtv1.VirtualMachineSpec{
			DataVolumeTemplates: []virtv1.DataVolumeTemplateSpec{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: dvName,
					},
					Spec: cdiv1beta1.DataVolumeSpec{
						Source: &cdiv1beta1.DataVolumeSource{
							Blank: &cdiv1beta1.DataVolumeBlankImage{},
						},
						Storage: &cdiv1beta1.StorageSpec{
							Resources: corev1.VolumeResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			Instancetype: &virtv1.InstancetypeMatcher{
				Name: "u1.small",
			},
			Preference: &virtv1.PreferenceMatcher{
				Name: "fedora",
			},
			RunStrategy: ptr.To(virtv1.RunStrategyAlways),
			Template: &virtv1.VirtualMachineInstanceTemplateSpec{
				Spec: virtv1.VirtualMachineInstanceSpec{
					Domain: virtv1.DomainSpec{
						Devices: virtv1.Devices{
							Interfaces: []virtv1.Interface{
								{
									Name:       "default",
									MacAddress: sourceMACAddress,
									InterfaceBindingMethod: virtv1.InterfaceBindingMethod{
										Masquerade: &virtv1.InterfaceMasquerade{},
									},
								},
							},
						},
						Firmware: &virtv1.Firmware{
							Serial: sourceSerial,
							UUID:   sourceUUID,
						},
					},
					Networks: []virtv1.Network{
						{
							Name:          "default",
							NetworkSource: virtv1.NetworkSource{Pod: &virtv1.PodNetwork{}},
						},
					},
					Volumes: []virtv1.Volume{
						{
							Name: "rootdisk",
							VolumeSource: virtv1.VolumeSource{
								DataVolume: &virtv1.DataVolumeSource{
									Name: dvName,
								},
							},
						},
					},
				},
			},
		},
	}
}

func decodeFunctestVM(raw []byte) *virtv1.VirtualMachine {
	var obj map[string]any
	ExpectWithOffset(1, json.Unmarshal(raw, &obj)).To(Succeed())
	vm := &virtv1.VirtualMachine{}
	ExpectWithOffset(1, runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj, vm, true)).To(Succeed())
	return vm
}

func newVMWithPersistentEFI() *virtv1.VirtualMachine {
	vm := newVM()
	vm.Name = "my-test-vm-efi-" + rand.String(5)
	vm.Spec.Template.Spec.Domain.Firmware = &virtv1.Firmware{
		Bootloader: &virtv1.Bootloader{
			EFI: &virtv1.EFI{
				Persistent: ptr.To(true),
			},
		},
	}
	return vm
}
