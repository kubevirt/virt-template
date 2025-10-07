package testing

import (
	"bytes"

	virttemplattectl "kubevirt.io/virt-template/internal/virttemplatectl"
)

func NewRepeatableVirttemplatectlCommand(args ...string) func() error {
	return func() error {
		cmd := virttemplattectl.NewVirtctlCommand()
		cmd.SetArgs(args)
		return cmd.Execute()
	}
}

func NewRepeatableVirttemplatectlCommandWithOut(args ...string) func() ([]byte, error) {
	return func() ([]byte, error) {
		out := &bytes.Buffer{}
		cmd := virttemplattectl.NewVirtctlCommand()
		cmd.SetArgs(args)
		cmd.SetOut(out)
		err := cmd.Execute()
		return out.Bytes(), err
	}
}
