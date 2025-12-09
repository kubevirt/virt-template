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

func NewRepeatableVirttemplatectlCommandWithOutAndErr(args ...string) func() (out, errOut []byte, err error) {
	return func() ([]byte, []byte, error) {
		out := &bytes.Buffer{}
		errOut := &bytes.Buffer{}
		cmd := virttemplattectl.NewVirtctlCommand()
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(errOut)
		err := cmd.Execute()
		return out.Bytes(), errOut.Bytes(), err
	}
}
