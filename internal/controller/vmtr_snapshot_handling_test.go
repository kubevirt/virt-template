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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtv1 "kubevirt.io/api/core/v1"
	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/apimachinery"
	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest Controller VirtualMachineSnapshot handling", func() {
	var (
		reconciler *controller.VirtualMachineTemplateRequestReconciler
		tplReq     *v1alpha1.VirtualMachineTemplateRequest
		snap       *snapshotv1beta1.VirtualMachineSnapshot
	)

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{},
			Scheme:     k8sClient.Scheme(),
		}

		tplReq = createRequest(k8sClient, testNamespace, testVMNamespace)
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())

		snap = &snapshotv1beta1.VirtualMachineSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apimachinery.GetStableName(tplReq.Name, string(tplReq.UID)),
				Namespace: testVMNamespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(snap), snap)).To(Succeed())
	})

	It("should create snapshot with correct spec", func() {
		Expect(snap.Spec.Source.Name).To(Equal(testVMName))
		Expect(*snap.Spec.Source.APIGroup).To(Equal(virtv1.VirtualMachineGroupVersionKind.Group))
		Expect(snap.Spec.Source.Kind).To(Equal(virtv1.VirtualMachineGroupVersionKind.Kind))
		Expect(snap.Labels[v1alpha1.LabelRequestUID]).To(Equal(string(tplReq.UID)))
	})

	It("should not recreate snapshot if it already exists", func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		newSnap := &snapshotv1beta1.VirtualMachineSnapshot{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(snap), newSnap)).To(Succeed())

		Expect(newSnap.UID).To(Equal(snap.UID))
	})

	It("should fail when snapshot exists with wrong UID", func() {
		snap.Labels[v1alpha1.LabelRequestUID] = wrongUID
		Expect(k8sClient.Update(context.Background(), snap)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := MatchRegexp("virtualMachineSnapshot .* does not belong to this request")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	})
})
