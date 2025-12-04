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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest controller reconciliation control", func() {
	var reconciler *controller.VirtualMachineTemplateRequestReconciler

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{},
			Scheme:     k8sClient.Scheme(),
		}
	})

	It("should not reconcile when Progressing condition is False", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		const reason = "SomeReason"
		meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
			Type:   v1alpha1.ConditionProgressing,
			Status: metav1.ConditionFalse,
			Reason: reason,
		})
		Expect(k8sClient.Status().Update(context.Background(), tplReq)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		Expect(tplReq.Status.Conditions).To(HaveLen(1))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, reason)
	})

	It("should reconcile when no Progressing condition exists", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})

	It("should reconcile when Progressing condition is True", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
			Type:   v1alpha1.ConditionProgressing,
			Status: metav1.ConditionTrue,
			Reason: "SomeReason",
		})
		Expect(k8sClient.Status().Update(context.Background(), tplReq)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})
})
