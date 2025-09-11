package openapi

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"

	"kubevirt.io/virt-template/client-go/api"
)

var info = &spec.Info{
	InfoProps: spec.InfoProps{
		Title:       "KubeVirt Template API",
		Description: "This is KubeVirt Template API an add-on for Kubernetes.",
		Contact: &spec.ContactInfo{
			Name:  "kubevirt-dev",
			Email: "kubevirt-dev@googlegroups.com",
			URL:   "https://github.com/kubevirt/virt-template",
		},
		License: &spec.License{
			Name: "Apache 2.0",
			URL:  "https://www.apache.org/licenses/LICENSE-2.0",
		},
	},
}

func NewConfig(scheme *runtime.Scheme) *common.Config {
	return &common.Config{
		ProtocolList: []string{"https"},
		Info:         info,
		DefaultResponse: &spec.Response{
			ResponseProps: spec.ResponseProps{
				Description: "Default Response.",
			},
		},
		GetDefinitionName: openapi.NewDefinitionNamer(scheme).GetDefinitionName,
		GetDefinitions:    api.GetOpenAPIDefinitions,
	}
}

func NewV3Config(scheme *runtime.Scheme) *common.OpenAPIV3Config {
	config := &common.OpenAPIV3Config{
		Info: info,
		DefaultResponse: &spec3.Response{
			ResponseProps: spec3.ResponseProps{
				Description: "Default Response.",
			},
		},
		GetDefinitionName: openapi.NewDefinitionNamer(scheme).GetDefinitionName,
		GetDefinitions:    api.GetOpenAPIDefinitions,
	}
	config.Definitions = config.GetDefinitions(func(name string) spec.Ref {
		defName, _ := config.GetDefinitionName(name)
		return spec.MustCreateRef("#/components/schemas/" + common.EscapeJsonPointer(defName))
	})
	return config
}
