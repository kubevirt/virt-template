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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest Controller VirtualMachineSnapshotContent retrieval", func() {
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
		snap = setSnapshotStatus(k8sClient, snap, snapshotv1beta1.Succeeded, true, false)
	})

	It("should fail when VirtualMachineSnapshotContent does not exist but keep reconciling", func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := MatchRegexp("virtualmachinesnapshotcontents.snapshot.kubevirt.io .* not found")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})

	DescribeTable("should fail when VirtualMachineSnapshotContentName", func(name *string) {
		snap.Status.VirtualMachineSnapshotContentName = name
		Expect(k8sClient.Status().Update(context.Background(), snap)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := ContainSubstring("does not have a VirtualMachineSnapshotContentName")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	},
		Entry("is nil", nil),
		Entry("is empty", ptr.To("")),
	)
})
