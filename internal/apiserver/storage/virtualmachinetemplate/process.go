package virtualmachinetemplate

import (
	"context"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
	"kubevirt.io/virt-template/api/v1alpha1"
	templateclient "kubevirt.io/virt-template/client-go/template"

	"kubevirt.io/virt-template/internal/template"
)

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/status,verbs=get

const (
	// jsonBufferSize determines how far into the stream the decoder will look for JSON
	jsonBufferSize = 1024
	// debugLogLevel is the klog verbosity level for debug messages
	debugLogLevel = 5
)

type processor interface {
	Process(tpl *v1alpha1.VirtualMachineTemplate) (*virtv1.VirtualMachine, string, error)
}
type ProcessREST struct {
	client    templateclient.Interface
	processor processor
}

func NewProcessREST(client templateclient.Interface) *ProcessREST {
	return &ProcessREST{
		client:    client,
		processor: template.NewDefaultProcessor(),
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
		klog.V(debugLogLevel).Infof("POST /process for VirtualMachineTemplate %s/%s", ns, id)

		tpl, err := p.client.TemplateV1alpha1().VirtualMachineTemplates(ns).Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			r.Error(err)
			return
		}

		opts, err := decodeProcessOptions(req.Body)
		if err != nil {
			r.Error(err)
			return
		}

		tpl.Spec.Parameters, err = template.MergeParameters(tpl.Spec.Parameters, opts.Parameters)
		if err != nil {
			r.Error(err)
			return
		}

		vm, msg, err := p.processor.Process(tpl)
		if err != nil {
			r.Error(err)
			return
		}

		r.Object(
			http.StatusOK,
			&subresourcesv1alpha1.ProcessedVirtualMachineTemplate{
				TemplateRef: &corev1.ObjectReference{
					Namespace: ns,
					Name:      id,
				},
				VirtualMachine: vm,
				Message:        msg,
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

func decodeProcessOptions(body io.Reader) (*subresourcesv1alpha1.ProcessOptions, error) {
	opts := &subresourcesv1alpha1.ProcessOptions{}
	if err := yaml.NewYAMLOrJSONDecoder(body, jsonBufferSize).Decode(opts); err != nil {
		return nil, err
	}
	return opts, nil
}
