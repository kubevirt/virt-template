package virtualmachinetemplate

import (
	"context"
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
	"kubevirt.io/virt-template/client-go/template"
)

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/status,verbs=get

type ProcessREST struct {
	client template.Interface
}

func NewProcessREST(client template.Interface) *ProcessREST {
	return &ProcessREST{
		client: client,
	}
}

var (
	_ = rest.Storage(&ProcessREST{})
	_ = rest.Connecter(&ProcessREST{})
)

func (p *ProcessREST) New() runtime.Object {
	return &subresourcesv1alpha1.ProcessedVirtualMachineTemplate{}
}

func (p *ProcessREST) Destroy() {}

func (p *ProcessREST) Connect(ctx context.Context, id string, _ runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing namespace")
	}

	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		klog.Infof("POST /process for VirtualMachineTemplate %s/%s", ns, id)

		const jsonBufferSize = 1024
		processOptions := &subresourcesv1alpha1.ProcessOptions{}
		if err := yaml.NewYAMLOrJSONDecoder(req.Body, jsonBufferSize).Decode(processOptions); err != nil {
			r.Error(err)
			return
		}

		r.Object(
			http.StatusOK,
			&subresourcesv1alpha1.ProcessedVirtualMachineTemplate{
				VirtualMachine: &virtv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name: "something",
					},
				},
			},
		)
	}), nil
}

func (p *ProcessREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (p *ProcessREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}
