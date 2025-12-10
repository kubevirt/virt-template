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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest Controller VirtualMachineSnapshotContent readiness", func() {
	var (
		reconciler  *controller.VirtualMachineTemplateRequestReconciler
		tplReq      *v1alpha1.VirtualMachineTemplateRequest
		snapContent *snapshotv1beta1.VirtualMachineSnapshotContent
	)

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{},
			Scheme:     k8sClient.Scheme(),
		}

		tplReq = createRequest(k8sClient, testNamespace, testVMNamespace)
		snap := createSnapshot(k8sClient, tplReq)
		snap = setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
		snapContent = createSnapshotContent(k8sClient, snap)
	})

	DescribeTable("should wait and requeue after 10 seconds when snapshot content is not ready", func(setStatus bool) {
		if setStatus {
			setSnapshotContentStatus(k8sClient, snapContent, false)
		}

		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(10 * time.Second))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			ContainSubstring("Waiting for VirtualMachineSnapshotContent"))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonWaiting)
	},
		Entry("when status is not set", false),
		Entry("when ReadyToUse is false", true),
	)

	It("should fail when volume backup is missing VolumeSnapshotName", func() {
		snapContent.Spec.VolumeBackups[0].VolumeSnapshotName = nil
		Expect(k8sClient.Update(context.Background(), snapContent)).To(Succeed())

		setSnapshotContentStatus(k8sClient, snapContent, true)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := MatchRegexp("virtualMachineSnapshotContent .* is missing a VolumeSnapshotName for volume .*")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	})
})
