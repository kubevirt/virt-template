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

package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest controller CRD validation", func() {
	Context("through API server", func() {
		It("should reject creation when VirtualMachineRef.Namespace is empty", func() {
			tplReq := &v1alpha1.VirtualMachineTemplateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-empty-ns",
					Namespace: testNamespace,
				},
				Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
					VirtualMachineRef: v1alpha1.VirtualMachineReference{
						Namespace: "",
						Name:      testVMName,
					},
				},
			}
			err := k8sClient.Create(context.Background(), tplReq)
			Expect(err).To(MatchError(ContainSubstring("spec.virtualMachineRef.namespace: Required value")))
		})

		It("should reject creation when VirtualMachineRef.Name is empty", func() {
			tplReq := &v1alpha1.VirtualMachineTemplateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-empty-name",
					Namespace: testNamespace,
				},
				Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
					VirtualMachineRef: v1alpha1.VirtualMachineReference{
						Namespace: testVMNamespace,
						Name:      "",
					},
				},
			}
			err := k8sClient.Create(context.Background(), tplReq)
			Expect(err).To(MatchError(ContainSubstring("spec.virtualMachineRef.name: Required value")))
		})

		It("should reject updates to spec", func() {
			tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

			tplReq.Spec.VirtualMachineRef.Name = "different-vm"
			err := k8sClient.Update(context.Background(), tplReq)
			Expect(err).To(MatchError(ContainSubstring("spec is immutable")))
		})
	})

	Context("through Controller", func() {
		var (
			fakeClient client.Client
			reconciler *controller.VirtualMachineTemplateRequestReconciler
		)

		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().
				WithScheme(testScheme).
				WithStatusSubresource(&v1alpha1.VirtualMachineTemplateRequest{}).
				WithStatusSubresource(&snapshotv1beta1.VirtualMachineSnapshot{}).
				WithStatusSubresource(&snapshotv1beta1.VirtualMachineSnapshotContent{}).
				WithStatusSubresource(&cdiv1beta1.DataVolume{}).
				Build()

			reconciler = &controller.VirtualMachineTemplateRequestReconciler{
				Client:     fakeClient,
				VirtClient: &fakeKubevirtClient{},
				Scheme:     fakeClient.Scheme(),
			}
		})

		It("should fail request when VirtualMachineRef.Namespace is empty", func() {
			tplReq := &v1alpha1.VirtualMachineTemplateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-empty-ns",
					Namespace: testNamespace,
				},
				Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
					VirtualMachineRef: v1alpha1.VirtualMachineReference{
						Namespace: "",
						Name:      testVMName,
					},
				},
			}
			Expect(fakeClient.Create(context.Background(), tplReq)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(tplReq),
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
			expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonInvalidConfiguration,
				Equal("virtualMachineRef.namespace cannot be empty"))
			expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonInvalidConfiguration)
		})

		It("should fail request when VirtualMachineRef.Name is empty", func() {
			tplReq := &v1alpha1.VirtualMachineTemplateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-empty-name",
					Namespace: testNamespace,
				},
				Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
					VirtualMachineRef: v1alpha1.VirtualMachineReference{
						Namespace: testVMNamespace,
						Name:      "",
					},
				},
			}
			Expect(fakeClient.Create(context.Background(), tplReq)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(tplReq),
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
			expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonInvalidConfiguration,
				Equal("virtualMachineRef.name cannot be empty"))
			expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonInvalidConfiguration)
		})

		It("should fail when source VirtualMachine has no template spec", func() {
			tplReq := createRequest(fakeClient, testNamespace, testVMNamespace)
			snap := createSnapshot(fakeClient, tplReq)
			setSnapshotStatus(fakeClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
			snapContent := createSnapshotContent(fakeClient, snap)
			snapContent.Spec.Source.VirtualMachine.Spec.Template = nil
			Expect(fakeClient.Update(context.Background(), snapContent)).To(Succeed())
			setSnapshotContentStatus(fakeClient, snapContent, true)
			dv := createDataVolume(fakeClient, tplReq)
			setDataVolumeStatus(fakeClient, dv, cdiv1beta1.Succeeded, true, false)

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(tplReq),
			})
			matcher := MatchRegexp("source VirtualMachine .* has no template spec")
			Expect(err).To(MatchError(matcher))

			Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
			expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
			expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		})
	})
})
