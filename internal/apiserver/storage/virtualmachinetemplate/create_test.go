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

package virtualmachinetemplate_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/endpoints/request"

	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	virttemplatefake "kubevirt.io/virt-template-client-go/virttemplate/fake"

	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
)

var _ = Describe("CreateREST", func() {
	var (
		createREST     *virtualmachinetemplate.CreateREST
		fakeClient     *virttemplatefake.Clientset
		fakeVirtClient *fakeKubevirtClient
	)

	BeforeEach(func() {
		fakeClient = virttemplatefake.NewSimpleClientset(newVirtualMachineTemplate())
		fakeVirtClient = &fakeKubevirtClient{}
		createREST = virtualmachinetemplate.NewCreateREST(fakeClient, fakeVirtClient)
	})

	It("NewCreateREST should create a new CreateREST instance", func() {
		Expect(createREST).ToNot(BeNil())
	})

	It("New should return a ProcessedVirtualMachineTemplate object", func() {
		obj := createREST.New()
		Expect(obj).ToNot(BeNil())
		_, ok := obj.(*subresourcesv1alpha1.ProcessedVirtualMachineTemplate)
		Expect(ok).To(BeTrue())
	})

	It("Destroy should not panic", func() {
		Expect(func() { createREST.Destroy() }).ToNot(Panic())
	})

	It("NewConnectOptions should return empty options", func() {
		options, include, path := createREST.NewConnectOptions()
		Expect(options).To(BeNil())
		Expect(include).To(BeFalse())
		Expect(path).To(BeEmpty())
	})

	It("ConnectMethods should return POST method only", func() {
		Expect(createREST.ConnectMethods()).To(ConsistOf(http.MethodPost))
	})

	Context("Connect", func() {
		var (
			ctx       context.Context
			responder *fakeResponder
		)

		BeforeEach(func() {
			ctx = request.WithNamespace(context.Background(), testNamespace)
			responder = &fakeResponder{}
		})

		It("should return error when namespace is missing from context", func() {
			handler, err := createREST.Connect(context.Background(), testTemplateName, nil, nil)
			Expect(err).To(MatchError("missing namespace"))
			Expect(handler).To(BeNil())
		})

		It("should return a valid http.Handler when namespace is present", func() {
			handler, err := createREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())
			Expect(handler).ToNot(BeNil())
		})

		It("should process template and create VM successfully", func() {
			handler, err := createREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())

			invokeHandler(handler, nil)
			processed := expectSuccessfulProcess(responder)
			Expect(fakeVirtClient.createdVM).To(Equal(processed.VirtualMachine))
		})

		It("should return error when template is not found", func() {
			handler, err := createREST.Connect(ctx, "nonexistent", nil, responder)
			Expect(err).ToNot(HaveOccurred())

			invokeHandler(handler, nil)
			Expect(responder.err).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error when VM creation fails", func() {
			fakeVirtClient.createErr = context.DeadlineExceeded

			handler, err := createREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())

			invokeHandler(handler, nil)
			Expect(responder.err).To(MatchError(ContainSubstring(context.DeadlineExceeded.Error())))
		})
	})
})
