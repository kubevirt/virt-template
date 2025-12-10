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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/rand"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	virtv1 "kubevirt.io/api/core/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	templatesubresourcesv1alpha1 "kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	templatev1alpha1 "kubevirt.io/virt-template-api/core/v1alpha1"
	templateclient "kubevirt.io/virt-template-client-go/virttemplate"
)

var _ = Describe("VirtualMachineTemplateRequest", func() {
	var tplClient templateclient.Interface

	BeforeEach(func() {
		var err error

		tplClient, err = templateclient.NewForConfig(virtClient.Config())
		Expect(err).NotTo(HaveOccurred())
	})

	It("should create a VirtualMachineTemplate from an existing VirtualMachine", func() {
		vm, err := virtClient.VirtualMachine(NamespaceSecondaryTest).Create(context.Background(), newVM(), metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			vm, err = virtClient.VirtualMachine(NamespaceSecondaryTest).Get(context.Background(), vm.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(vm.Status.Ready).To(BeTrue())
		}, 5*time.Minute, 1*time.Second).Should(Succeed())

		tplReq := &templatev1alpha1.VirtualMachineTemplateRequest{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "virt-template-",
				Namespace:    NamespaceTest,
			},
			Spec: templatev1alpha1.VirtualMachineTemplateRequestSpec{
				VirtualMachineRef: templatev1alpha1.VirtualMachineReference{
					Namespace: vm.Namespace,
					Name:      vm.Name,
				},
			},
		}

		tplReq, err = tplClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
			Create(context.Background(), tplReq, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func(g Gomega) {
			tplReq, err = tplClient.TemplateV1alpha1().VirtualMachineTemplateRequests(NamespaceTest).
				Get(context.Background(), tplReq.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			cond := meta.FindStatusCondition(tplReq.Status.Conditions, templatev1alpha1.ConditionReady)
			g.Expect(cond).ToNot(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		}, 5*time.Minute, 1*time.Second).Should(Succeed())

		tpl, err := tplClient.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).
			Get(context.Background(), tplReq.Status.TemplateRef.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		name := "my-created-vm-" + rand.String(5)
		processedVM, err := tplClient.TemplateV1alpha1().VirtualMachineTemplates(NamespaceTest).CreateVirtualMachine(
			context.Background(), tpl.Name,
			templatesubresourcesv1alpha1.ProcessOptions{
				Parameters: map[string]string{
					"NAME": name,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(processedVM.VirtualMachine.Name).To(Equal(name))

		Eventually(func(g Gomega) {
			newVM, err := virtClient.VirtualMachine(NamespaceTest).Get(context.Background(), processedVM.VirtualMachine.Name, metav1.GetOptions{})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(newVM.Status.Ready).To(BeTrue())
		}, 5*time.Minute, 1*time.Second).Should(Succeed())
	})
})

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
					Domain: virtv1.DomainSpec{},
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
