package main

import (
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	_ "k8s.io/client-go/plugin/pkg/client/auth" // Import to initialize client auth plugins.

	"kubevirt.io/client-go/kubecli"

	templateapi "kubevirt.io/virt-template/api"
	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
	"kubevirt.io/virt-template/client-go/template"

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
	client, err := template.NewForConfig(virtClient.Config())
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	scheme := templatescheme.New()
	apiGroups := apiserver.APIGroups{
		subresourcesv1alpha1.GroupVersion: {
			templateapi.PluralResourceName:              virtualmachinetemplate.NewDummyREST(),
			templateapi.PluralResourceName + "/process": virtualmachinetemplate.NewProcessREST(client),
		},
	}

	if err := s.Run(
		"virt-template-apiserver",
		scheme, openapi.NewConfig(scheme), openapi.NewV3Config(scheme), apiGroups,
	); err != nil {
		os.Exit(1)
	}
}
