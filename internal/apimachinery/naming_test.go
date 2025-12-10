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

package apimachinery_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/validation"

	"kubevirt.io/virt-template/internal/apimachinery"
)

var _ = Describe("GetStableName", func() {
	const (
		myResource = "my-resource"
		resourceA  = "resource-a"
		resourceB  = "resource-b"
		input1     = "input1"
		input2     = "input2"
		prefixX    = "x-"
		prefixObj  = "obj-"
	)

	It("should generate a valid DNS-1035 label", func() {
		name := apimachinery.GetStableName(myResource, input1, input2)
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should generate deterministic names", func() {
		name1 := apimachinery.GetStableName(myResource, input1, input2)
		name2 := apimachinery.GetStableName(myResource, input1, input2)
		Expect(name1).To(Equal(name2))
	})

	It("should generate different names for different inputs", func() {
		name1 := apimachinery.GetStableName(myResource, input1)
		name2 := apimachinery.GetStableName(myResource, input2)
		Expect(name1).ToNot(Equal(name2))
	})

	It("should generate different names for different base strings", func() {
		name1 := apimachinery.GetStableName(resourceA, input1)
		name2 := apimachinery.GetStableName(resourceB, input1)
		Expect(name1).ToNot(Equal(name2))
	})

	It("should prefix with 'x-' when base is empty", func() {
		name := apimachinery.GetStableName("", input1)
		Expect(name).To(HavePrefix(prefixX))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should prefix with 'x-' when base starts with a number", func() {
		name := apimachinery.GetStableName("123-resource", input1)
		Expect(name).To(HavePrefix(prefixX))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	DescribeTable("should truncate long base names", func(base string) {
		name := apimachinery.GetStableName(base, input1)
		Expect(len(name)).To(BeNumerically("<=", validation.DNS1035LabelMaxLength))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	},
		Entry("base at max length", strings.Repeat("a", apimachinery.MaxGeneratedNameLength)),
		Entry("base exceeding max length", strings.Repeat("b", 100)),
	)

	It("should remove trailing hyphens after truncation", func() {
		base := strings.Repeat("a", apimachinery.MaxGeneratedNameLength-1) + "-"
		name := apimachinery.GetStableName(base, input1)
		Expect(name).ToNot(ContainSubstring("--"))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should fallback to 'obj-' when base has uppercase letter", func() {
		name := apimachinery.GetStableName("Resource", input1)
		Expect(name).To(HavePrefix(prefixObj))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should fallback to 'obj-' prefix for garbage base", func() {
		name := apimachinery.GetStableName("!!!", input1)
		Expect(name).To(HavePrefix(prefixObj))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should handle base with only special characters", func() {
		name := apimachinery.GetStableName("@#$%", input1)
		Expect(name).To(HavePrefix(prefixObj))
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should handle no additional inputs", func() {
		name := apimachinery.GetStableName(myResource)
		Expect(name).ToNot(BeEmpty())
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should handle multiple additional inputs", func() {
		name := apimachinery.GetStableName(myResource, "a", "b", "c", "d", "e")
		Expect(name).ToNot(BeEmpty())
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty())
	})

	It("should include hash suffix", func() {
		name := apimachinery.GetStableName(myResource, input1)
		parts := strings.Split(name, "-")
		Expect(parts).To(HaveLen(3))
		Expect(parts[2]).To(HaveLen(apimachinery.HashLength))
	})

	DescribeTable("should generate valid DNS-1035 labels for edge cases", func(base string, inputs ...string) {
		name := apimachinery.GetStableName(base, inputs...)
		Expect(name).ToNot(BeEmpty())
		Expect(validation.IsDNS1035Label(name)).To(BeEmpty(),
			"expected valid DNS-1035 label, got: %s", name)
	},
		Entry("single character base", "a"),
		Entry("base starting with hyphen", "-resource"),
		Entry("base ending with hyphen", "resource-"),
		Entry("numeric base", "12345"),
		Entry("unicode base", "ресурс"),
	)
})
