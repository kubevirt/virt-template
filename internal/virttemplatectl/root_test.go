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
