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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/endpoints/request"

	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	virttemplatefake "kubevirt.io/virt-template-client-go/virttemplate/fake"

	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
)

var _ = Describe("ProcessREST", func() {
	var (
		processREST *virtualmachinetemplate.ProcessREST
		fakeClient  *virttemplatefake.Clientset
	)

	BeforeEach(func() {
		fakeClient = virttemplatefake.NewSimpleClientset(newVirtualMachineTemplate())
		processREST = virtualmachinetemplate.NewProcessREST(fakeClient)
	})

	It("NewProcessREST should create a new ProcessREST instance", func() {
		Expect(processREST).ToNot(BeNil())
	})

	It("New should return a ProcessedVirtualMachineTemplate object", func() {
		obj := processREST.New()
		Expect(obj).ToNot(BeNil())
		_, ok := obj.(*subresourcesv1alpha1.ProcessedVirtualMachineTemplate)
		Expect(ok).To(BeTrue())
	})

	It("Destroy should not panic", func() {
		Expect(func() { processREST.Destroy() }).ToNot(Panic())
	})

	It("NewConnectOptions should return nil options", func() {
		options, include, path := processREST.NewConnectOptions()
		Expect(options).To(BeNil())
		Expect(include).To(BeFalse())
		Expect(path).To(BeEmpty())
	})

	It("ConnectMethods should return POST method only", func() {
		Expect(processREST.ConnectMethods()).To(ConsistOf(http.MethodPost))
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
			handler, err := processREST.Connect(context.Background(), testTemplateName, nil, nil)
			Expect(err).To(MatchError("missing namespace"))
			Expect(handler).To(BeNil())
		})

		It("should return a valid http.Handler when namespace is present", func() {
			handler, err := processREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())
			Expect(handler).ToNot(BeNil())
		})

		It("should process template successfully", func() {
			handler, err := processREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())

			invokeHandler(handler, nil)
			expectSuccessfulProcess(responder)
		})

		It("should return error when template is not found", func() {
			handler, err := processREST.Connect(ctx, "nonexistent", nil, responder)
			Expect(err).ToNot(HaveOccurred())

			invokeHandler(handler, nil)
			Expect(responder.err).To(MatchError(ContainSubstring("not found")))
		})

		It("should return error for invalid request body", func() {
			handler, err := processREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json{")))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			Expect(responder.err).To(MatchError(ContainSubstring("error unmarshaling JSON")))
		})

		It("should merge provided parameters with template parameters", func() {
			handler, err := processREST.Connect(ctx, testTemplateName, nil, responder)
			Expect(err).ToNot(HaveOccurred())

			const overriddenName = "overriddenName"
			invokeHandler(handler, &subresourcesv1alpha1.ProcessOptions{
				Parameters: map[string]string{
					testParamName: overriddenName,
				},
			})

			Expect(responder.statusCode).To(Equal(http.StatusOK))
			processed, ok := responder.obj.(*subresourcesv1alpha1.ProcessedVirtualMachineTemplate)
			Expect(ok).To(BeTrue())
			Expect(processed.VirtualMachine.Name).To(Equal(overriddenName))
		})
	})
})
