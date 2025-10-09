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

package virttemplattectl

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"

	"kubevirt.io/virt-template/internal/virttemplatectl/clientconfig"
	"kubevirt.io/virt-template/internal/virttemplatectl/convert"
	"kubevirt.io/virt-template/internal/virttemplatectl/process"
	"kubevirt.io/virt-template/internal/virttemplatectl/templates"
)

var (
	NewVirtctlCommand = NewVirttemplatectlCommandFn

	programName = GetProgramName(filepath.Base(os.Args[0]))
)

// GetProgramName returns the command name to display in help texts.
// If `virttemplatectl` is installed via krew to be used as a kubectl plugin, it's run via a symlink, so the basename then
// is `kubectl-virttemplate`. In this case we want to accommodate the user by adjusting the help text (usage, examples and
// the like) by displaying `kubectl virttemplate <command>` instead of `virttemplatectl <command>`.
// see https://github.com/kubevirt/kubevirt/issues/2356 for more details
// see also templates.go
func GetProgramName(binary string) string {
	if strings.HasSuffix(binary, "-virttemplate") {
		return strings.TrimSuffix(binary, "-virttemplate") + " virttemplate"
	}
	return "virttemplatectl"
}

func NewVirttemplatectlCommandFn() *cobra.Command {
	// used in cobra templates to display either `kubectl virttemplate` or `virttemplatectl`
	cobra.AddTemplateFunc(
		"ProgramName", func() string {
			return programName
		},
	)

	// used to enable replacement of `ProgramName` placeholder for cobra.Example, which has no template support
	cobra.AddTemplateFunc(
		"prepare", func(s string) string {
			return strings.ReplaceAll(s, "{{ProgramName}}", programName)
		},
	)

	optionsCmd := &cobra.Command{
		Use:    "options",
		Hidden: true,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Print(cmd.UsageString())
		},
	}
	optionsCmd.SetUsageTemplate(templates.OptionsUsageTemplate())

	rootCmd := &cobra.Command{
		Use:           programName,
		Short:         programName + " controls virtual machine template related operations on your kubernetes cluster.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Print(cmd.UsageString())
		},
	}
	addVerbosityFlag(rootCmd.PersistentFlags())
	rootCmd.SetUsageTemplate(templates.MainUsageTemplate())
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetContext(clientconfig.NewContext(
		context.Background(), kubecli.DefaultClientConfig(rootCmd.PersistentFlags()),
	))

	rootCmd.AddCommand(
		optionsCmd,
		process.NewProcessCommand(),
		convert.NewConvertCommand(),
	)

	return rootCmd
}

func addVerbosityFlag(fs *pflag.FlagSet) {
	// The verbosity flag is added to the default flag set
	// by init() in staging/src/kubevirt.io/client-go/log/log.go.
	// We re-add it here to make it available in virtctl commands.
	if f := flag.CommandLine.Lookup("v"); f != nil {
		fs.AddFlag(pflag.PFlagFromGoFlag(f))
	} else {
		panic("failed to find verbosity flag \"v\" in default flag set")
	}
}

func Execute() int {
	log.InitializeLogging(programName)
	cmd := NewVirtctlCommand()
	if err := cmd.Execute(); err != nil {
		cmd.PrintErrln(err)
		return 1
	}
	return 0
}
