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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templateapi "kubevirt.io/virt-template-api/core"
	"kubevirt.io/virt-template-api/core/subresourcesv1alpha1"

	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
)

var _ = Describe("DummyREST", func() {
	var dummy *virtualmachinetemplate.DummyREST

	BeforeEach(func() {
		dummy = virtualmachinetemplate.NewDummyREST()
	})

	It("NewDummyRest should create a new DummyREST instance", func() {
		Expect(dummy).ToNot(BeNil())
	})

	It("New should return a VirtualMachineTemplate object", func() {
		obj := dummy.New()
		Expect(obj).ToNot(BeNil())
		_, ok := obj.(*subresourcesv1alpha1.VirtualMachineTemplate)
		Expect(ok).To(BeTrue())
	})

	It("Destroy should not panic", func() {
		Expect(func() { dummy.Destroy() }).ToNot(Panic())
	})

	It("NamespaceScoped should return true", func() {
		Expect(dummy.NamespaceScoped()).To(BeTrue())
	})

	It("GetSingularNmae should return the singular resource name", func() {
		Expect(dummy.GetSingularName()).To(Equal(templateapi.SingularResourceName))
	})
})
