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

	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest Controller VirtualMachineSnapshotStatus sync", func() {
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
		snap = createSnapshot(k8sClient, tplReq)
	})

	It("should set Waiting condition while snapshot is not ready", func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			ContainSubstring("Waiting for VirtualMachineSnapshot"))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})

	It("should set Failed condition when snapshot fails", func() {
		setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Failed))

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed,
			MatchRegexp("VirtualMachineSnapshot .* failed"))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	})

	It("should wait when snapshot is in progress", func() {
		setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.InProgress), withProgressing())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			ContainSubstring("Waiting for VirtualMachineSnapshot"))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})

	It("should wait when snapshot phase is unset", func() {
		setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.PhaseUnset))

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			ContainSubstring("Waiting for VirtualMachineSnapshot"))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})

	It("should wait when snapshot phase is in progress", func() {
		setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.InProgress))

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			ContainSubstring("Waiting for VirtualMachineSnapshot"))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})
})
