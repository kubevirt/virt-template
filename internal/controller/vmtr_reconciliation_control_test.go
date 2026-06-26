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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/virt-template-api/core/v1beta1"

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
			Type:   v1beta1.ConditionProgressing,
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
		expectCondition(tplReq, v1beta1.ConditionProgressing, metav1.ConditionFalse, reason)
	})

	It("should reconcile when no Progressing condition exists", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1beta1.ConditionReady, metav1.ConditionFalse, v1beta1.ReasonWaiting)
		expectCondition(tplReq, v1beta1.ConditionProgressing, metav1.ConditionTrue, v1beta1.ReasonReconciling)
	})

	It("should reconcile when Progressing condition is True", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
			Type:   v1beta1.ConditionProgressing,
			Status: metav1.ConditionTrue,
			Reason: "SomeReason",
		})
		Expect(k8sClient.Status().Update(context.Background(), tplReq)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1beta1.ConditionReady, metav1.ConditionFalse, v1beta1.ReasonWaiting)
		expectCondition(tplReq, v1beta1.ConditionProgressing, metav1.ConditionTrue, v1beta1.ReasonReconciling)
	})

	DescribeTable(
		"TTL handling",
		func(ttl *int32, ready bool, readyAge time.Duration, expectDeleted, expectRequeue bool) {
			status := metav1.ConditionFalse
			reason := v1beta1.ReasonFailed
			if ready {
				status = metav1.ConditionTrue
				reason = v1beta1.ReasonReconciled
			}

			tplReq := &v1beta1.VirtualMachineTemplateRequest{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: testRequestPrefix,
					Namespace:    testNamespace,
				},
				Spec: v1beta1.VirtualMachineTemplateRequestSpec{
					VirtualMachineRef: v1beta1.VirtualMachineReference{
						Namespace: testVMNamespace,
						Name:      testVMName,
					},
					TTLSecondsAfterFinished: ttl,
				},
			}
			Expect(k8sClient.Create(context.Background(), tplReq)).To(Succeed())

			meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
				Type:               v1beta1.ConditionReady,
				Status:             status,
				Reason:             reason,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-readyAge)),
			})
			meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
				Type:   v1beta1.ConditionProgressing,
				Status: metav1.ConditionFalse,
				Reason: reason,
			})
			Expect(k8sClient.Status().Update(context.Background(), tplReq)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(tplReq),
			})
			Expect(err).ToNot(HaveOccurred())

			if expectDeleted {
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
				Expect(tplReq.DeletionTimestamp.IsZero()).To(BeFalse())

				_, err = reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(tplReq),
				})
				Expect(err).ToNot(HaveOccurred())
				err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)
				Expect(err).To(MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound"))
			} else if expectRequeue {
				Expect(result.RequeueAfter).To(BeNumerically(">", 0))
				Expect(result.RequeueAfter).To(BeNumerically("<=", readyAge))
				Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
			} else {
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
			}
		},
		Entry("should delete a succeeded VMTR when TTL has expired",
			ptr.To[int32](3600), true, 2*time.Hour, true, false),
		Entry("should requeue a succeeded VMTR when TTL has not expired",
			ptr.To[int32](7200), true, 1*time.Hour, false, true),
		Entry("should not delete a failed VMTR even when TTL is set",
			ptr.To[int32](3600), false, 2*time.Hour, false, false),
		Entry("should not delete a succeeded VMTR without TTL",
			(*int32)(nil), true, 2*time.Hour, false, false),
		Entry("should delete a succeeded VMTR immediately when TTL is zero",
			ptr.To[int32](0), true, 2*time.Hour, true, false),
		Entry("should delete a succeeded VMTR when TTL equals elapsed time",
			ptr.To[int32](3600), true, 1*time.Hour, true, false),
	)
})
