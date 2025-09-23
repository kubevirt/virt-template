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
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
	"kubevirt.io/virt-template/client-go/template"
)

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/status,verbs=get
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=create

type CreateREST struct {
	templateClient template.Interface
	virtClient     kubecli.KubevirtClient
}

func NewCreateREST(
	templateClient template.Interface,
	virtClient kubecli.KubevirtClient,
) *CreateREST {
	return &CreateREST{
		templateClient: templateClient,
		virtClient:     virtClient,
	}
}

var (
	_ = rest.Storage(&CreateREST{})
	_ = rest.Connecter(&CreateREST{})
)

func (c *CreateREST) New() runtime.Object {
	return &subresourcesv1alpha1.ProcessedVirtualMachineTemplate{}
}

func (c *CreateREST) Destroy() {}

//nolint:dupl
func (c *CreateREST) Connect(ctx context.Context, id string, _ runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing namespace")
	}

	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		klog.Infof("POST /create for VirtualMachineTemplate %s/%s", ns, id)

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

func (c *CreateREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (c *CreateREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}
