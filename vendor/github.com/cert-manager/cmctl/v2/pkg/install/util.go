/*
Copyright 2021 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package install

import (
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/values"
	"k8s.io/client-go/util/homedir"
)

func addInstallFlags(f *pflag.FlagSet, client *action.Install) {
	f.StringVar(&client.ReleaseName, "release-name", "cert-manager", "Name of the helm release")
	if err := f.MarkHidden("release-name"); err != nil {
		panic(err)
	}
	f.BoolVarP(&client.GenerateName, "generate-name", "g", false, "Generate the name (instead of using the default 'cert-manager' value)")
	if err := f.MarkHidden("generate-name"); err != nil {
		panic(err)
	}
	f.StringVar(&client.NameTemplate, "name-template", "", "Specify template used to name the release")
	if err := f.MarkHidden("name-template"); err != nil {
		panic(err)
	}
	f.StringVar(&client.Description, "description", "cert-manager was installed using the cert-manager CLI", "Add a custom description")
	if err := f.MarkHidden("description"); err != nil {
		panic(err)
	}
}

func addValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{}, "Specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.StringValues, "set-string", []string{}, "Set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	if err := f.MarkHidden("set-string"); err != nil {
		panic(err)
	}
	f.StringArrayVar(&v.FileValues, "set-file", []string{}, "Set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	if err := f.MarkHidden("set-file"); err != nil {
		panic(err)
	}
}

// defaultKeyring returns the expanded path to the default keyring.
func defaultKeyring() string {
	if v, ok := os.LookupEnv("GNUPGHOME"); ok {
		return filepath.Join(v, "pubring.gpg")
	}
	return filepath.Join(homedir.HomeDir(), ".gnupg", "pubring.gpg")
}

func addChartPathOptionsFlags(f *pflag.FlagSet, c *action.ChartPathOptions) {
	c.Keyring = defaultKeyring()
	c.RepoURL = "https://charts.jetstack.io"
	f.StringVar(&c.Version, "version", "", "specify a version constraint for the chart version to use. This constraint can be a specific tag (e.g. 1.1.1) or it may reference a valid range (e.g. ^2.0.0). If this is not specified, the latest version is used")
}
