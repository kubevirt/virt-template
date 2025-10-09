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

package main

import (
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // Import to initialize client auth plugins.

	"kubevirt.io/client-go/kubecli"

	templateapi "kubevirt.io/virt-template/api"
	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
	templateclient "kubevirt.io/virt-template/client-go/template"

	"kubevirt.io/virt-template/internal/apiserver"
	"kubevirt.io/virt-template/internal/apiserver/openapi"
	"kubevirt.io/virt-template/internal/apiserver/storage/virtualmachinetemplate"
	templatescheme "kubevirt.io/virt-template/internal/scheme"
)

func main() {
	s := apiserver.New()

	s.AddFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(kubecli.FlagSet())
	pflag.Parse()

	virtClient, err := kubecli.GetKubevirtClient()
	if err != nil {
		klog.Fatalf("Failed to get virtClient: %v", err)
	}
	client, err := templateclient.NewForConfig(virtClient.Config())
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	scheme := templatescheme.New()
	apiGroups := apiserver.APIGroups{
		subresourcesv1alpha1.GroupVersion: {
			templateapi.PluralResourceName:              virtualmachinetemplate.NewDummyREST(),
			templateapi.PluralResourceName + "/process": virtualmachinetemplate.NewProcessREST(client),
			templateapi.PluralResourceName + "/create":  virtualmachinetemplate.NewCreateREST(client, virtClient),
		},
	}

	if err := s.Run(
		"virt-template-apiserver",
		scheme, openapi.NewConfig(scheme), openapi.NewV3Config(scheme), apiGroups,
	); err != nil {
		os.Exit(1)
	}
}
