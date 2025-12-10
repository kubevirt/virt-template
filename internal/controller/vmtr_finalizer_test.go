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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest controller finalizer management and deletion handling", func() {
	var reconciler *controller.VirtualMachineTemplateRequestReconciler

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{},
			Scheme:     k8sClient.Scheme(),
		}
	})

	It("should return without error when request does not exist", func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "non-existent",
				Namespace: testNamespace,
			},
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should add finalizer on first reconcile", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)
		Expect(controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)).To(BeFalse())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		Expect(controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)).To(BeTrue())
	})

	It("should not add duplicate finalizers", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)
		Expect(controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)).To(BeFalse())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		Expect(controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)).To(BeTrue())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		count := 0
		for _, f := range tplReq.Finalizers {
			if f == v1alpha1.FinalizerSnapshotCleanup {
				count++
			}
		}
		Expect(count).To(Equal(1))
	})

	It("should remove finalizer on deletion", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		Expect(controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)).To(BeTrue())

		Expect(k8sClient.Delete(context.Background(), tplReq)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)
		Expect(err).To(MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound"))
	})

	It("should clean up snapshot on deletion", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)
		snap := createSnapshot(k8sClient, tplReq)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Delete(context.Background(), tplReq)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(snap), snap)
		Expect(err).To(MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound"))
	})
})
