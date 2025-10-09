package virtualmachinetemplate

import (
	"context"
	"fmt"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
	templateclient "kubevirt.io/virt-template/client-go/template"

	"kubevirt.io/virt-template/internal/template"
)

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/status,verbs=get
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=create

type CreateREST struct {
	client     templateclient.Interface
	virtClient kubecli.KubevirtClient
	processor  processor
}

func NewCreateREST(
	client templateclient.Interface,
	virtClient kubecli.KubevirtClient,
) *CreateREST {
	return &CreateREST{
		client:     client,
		virtClient: virtClient,
		processor:  template.GetDefaultProcessor(),
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

func (c *CreateREST) Connect(ctx context.Context, id string, _ runtime.Object, r rest.Responder) (http.Handler, error) {
	ns, ok := request.NamespaceFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("missing namespace")
	}

	return http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		klog.V(debugLogLevel).Infof("POST /create for VirtualMachineTemplate %s/%s", ns, id)

		processed, err := processTemplate(ctx, c.client, c.processor, req.Body, ns, id)
		if err != nil {
			r.Error(err)
			return
		}

		processed.VirtualMachine, err = c.virtClient.VirtualMachine(ns).Create(ctx, processed.VirtualMachine, metav1.CreateOptions{})
		if err != nil {
			r.Error(apierrors.NewInternalError(fmt.Errorf("error creating VirtualMachine: %w", err)))
			return
		}

		r.Object(http.StatusOK, processed)
	}), nil
}

func (c *CreateREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (c *CreateREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}
