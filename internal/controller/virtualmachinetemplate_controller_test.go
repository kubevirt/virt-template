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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplate Controller", func() {
	var (
		reconciler *controller.VirtualMachineTemplateReconciler
		tpl        *v1alpha1.VirtualMachineTemplate
	)

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	AfterEach(func() {
		if tpl != nil {
			Expect(k8sClient.Delete(context.Background(), tpl)).To(Or(Succeed(), MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound")))
		}
	})

	It("should set the Ready condition", func() {
		By("Creating a new VirtualMachineTemplate")
		tpl = &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-template",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), tpl)).To(Succeed())

		By("Reconciling the created VirtualMachineTemplate")
		namespacedName := types.NamespacedName{
			Name:      tpl.Name,
			Namespace: metav1.NamespaceDefault,
		}
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: namespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), namespacedName, tpl)).To(Succeed())
		Expect(tpl.Status.Conditions).To(HaveLen(1))
		Expect(tpl.Status.Conditions[0].Type).To(Equal(v1alpha1.ConditionReady))
		Expect(tpl.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
		Expect(tpl.Status.Conditions[0].Reason).To(Equal(v1alpha1.ReasonReconciled))
		Expect(tpl.Status.Conditions[0].Message).To(Equal("VirtualMachineTemplate is ready to be processed"))
		Expect(tpl.Status.Conditions[0].ObservedGeneration).To(Equal(tpl.Generation))
	})
})
