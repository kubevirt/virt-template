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

package virttemplattectl_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	virttemplattectl "kubevirt.io/virt-template/internal/virttemplatectl"
	"kubevirt.io/virt-template/internal/virttemplatectl/testing"
)

var _ = Describe("virtctl", func() {
	DescribeTable("GetProgramName", func(binary, expected string) {
		Expect(virttemplattectl.GetProgramName(binary)).To(Equal(expected))
	},
		Entry("returns virtctl", "virttemplatectl", "virttemplatectl"),
		Entry("returns virtctl as default", "42", "virttemplatectl"),
		Entry("returns kubectl", "kubectl-virttemplate", "kubectl virttemplate"),
		Entry("returns oc", "oc-virttemplate", "oc virttemplate"),
	)

	DescribeTable("the log verbosity flag should be supported", func(arg string) {
		Expect(testing.NewRepeatableVirttemplatectlCommand(arg)()).To(Succeed())
	},
		Entry("regular flag", "--v=2"),
		Entry("shorthand flag", "-v=2"),
	)
})
