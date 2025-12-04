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

	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/controller"
)

const (
	testRequestName  = "test-request"
	testTemplateName = "test-template"
	fakeUID          = "test-uid-12345"
)

var _ = Describe("VirtualMachineTemplateRequest controller EnqueueRequestByUID", func() {
	var (
		fakeClient client.Client
		reconciler *controller.VirtualMachineTemplateRequestReconciler
	)

	BeforeEach(func() {
		fakeClient = fake.NewClientBuilder().
			WithScheme(testScheme).
			WithIndex(&v1alpha1.VirtualMachineTemplateRequest{}, "metadata.uid", func(obj client.Object) []string {
				return []string{string(obj.GetUID())}
			}).
			Build()

		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client: fakeClient,
			Scheme: fakeClient.Scheme(),
		}
	})

	It("should return nil when object has no LabelRequestUID", func() {
		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testTemplateName,
				Namespace: testNamespace,
			},
		}

		requests := reconciler.EnqueueRequestByUID(context.Background(), tpl)
		Expect(requests).To(BeNil())
	})

	It("should return nil when no matching request is found", func() {
		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testTemplateName,
				Namespace: testNamespace,
				Labels: map[string]string{
					v1alpha1.LabelRequestUID: "non-existent-uid",
				},
			},
		}

		requests := reconciler.EnqueueRequestByUID(context.Background(), tpl)
		Expect(requests).To(BeNil())
	})

	DescribeTable("should enqueue request when matching request is found", func(obj client.Object) {
		tplReq := fakeRequest()
		Expect(fakeClient.Create(context.Background(), tplReq)).To(Succeed())

		requests := reconciler.EnqueueRequestByUID(context.Background(), obj)
		Expect(requests).To(HaveLen(1))
		Expect(requests[0].Namespace).To(Equal(tplReq.Namespace))
		Expect(requests[0].Name).To(Equal(tplReq.Name))
	},
		Entry("from VirtualMachineTemplate",
			&v1alpha1.VirtualMachineTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testTemplateName,
					Namespace: testNamespace,
					Labels: map[string]string{
						v1alpha1.LabelRequestUID: fakeUID,
					},
				},
			},
		),
		Entry("from VirtualMachineSnapshot",
			&snapshotv1beta1.VirtualMachineSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testSnapshotName,
					Namespace: testVMNamespace,
					Labels: map[string]string{
						v1alpha1.LabelRequestUID: fakeUID,
					},
				},
			},
		),
	)
})

func fakeRequest() *v1alpha1.VirtualMachineTemplateRequest {
	return &v1alpha1.VirtualMachineTemplateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRequestName,
			Namespace: testNamespace,
			UID:       fakeUID,
		},
	}
}
