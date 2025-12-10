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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	virtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	kvcorev1 "kubevirt.io/client-go/kubevirt/typed/core/v1"

	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"
	"kubevirt.io/virt-template-api/core/v1alpha1"
)

func TestVirtualMachineTemplateStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VirtualMachineTemplate Storage Suite")
}

const (
	testNamespace    = "test-namespace"
	testTemplateName = "test-template"
	testVMName       = "test-vm"
	testParamName    = "NAME"
	vmJSON           = `{"apiVersion":"kubevirt.io/v1","kind":"VirtualMachine","metadata":{"name":"${NAME}"}}`
)

type fakeResponder struct {
	statusCode int
	obj        runtime.Object
	err        error
}

func (f *fakeResponder) Object(statusCode int, obj runtime.Object) {
	f.statusCode = statusCode
	f.obj = obj
}

func (f *fakeResponder) Error(err error) {
	f.err = err
}

type fakeKubevirtClient struct {
	kubecli.KubevirtClient
	createErr error
	createdVM *virtv1.VirtualMachine
}

func (f *fakeKubevirtClient) VirtualMachine(_ string) kubecli.VirtualMachineInterface {
	return &fakeVirtualMachineInterface{
		createErr: f.createErr,
		createdVM: &f.createdVM,
	}
}

type fakeVirtualMachineInterface struct {
	kvcorev1.VirtualMachineInterface
	createErr error
	createdVM **virtv1.VirtualMachine
}

func (f *fakeVirtualMachineInterface) Create(
	_ context.Context, vm *virtv1.VirtualMachine, _ metav1.CreateOptions,
) (*virtv1.VirtualMachine, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	*f.createdVM = vm
	return vm, nil
}

func newVirtualMachineTemplate() *v1alpha1.VirtualMachineTemplate {
	return &v1alpha1.VirtualMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testTemplateName,
			Namespace: testNamespace,
		},
		Spec: v1alpha1.VirtualMachineTemplateSpec{
			VirtualMachine: &runtime.RawExtension{Raw: []byte(vmJSON)},
			Parameters: []v1alpha1.Parameter{
				{Name: testParamName, Value: testVMName},
			},
		},
	}
}

func invokeHandler(handler http.Handler, opts *subresourcesv1alpha1.ProcessOptions) {
	if opts == nil {
		opts = &subresourcesv1alpha1.ProcessOptions{}
	}
	body, err := json.Marshal(opts)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
}

func expectSuccessfulProcess(responder *fakeResponder) *subresourcesv1alpha1.ProcessedVirtualMachineTemplate {
	ExpectWithOffset(1, responder.statusCode).To(Equal(http.StatusOK))
	ExpectWithOffset(1, responder.obj).ToNot(BeNil())

	processed, ok := responder.obj.(*subresourcesv1alpha1.ProcessedVirtualMachineTemplate)
	ExpectWithOffset(1, ok).To(BeTrue())
	ExpectWithOffset(1, processed.TemplateRef.Name).To(Equal(testTemplateName))
	ExpectWithOffset(1, processed.TemplateRef.Namespace).To(Equal(testNamespace))
	ExpectWithOffset(1, processed.VirtualMachine).ToNot(BeNil())
	ExpectWithOffset(1, processed.VirtualMachine.Name).To(Equal(testVMName))

	return processed
}
