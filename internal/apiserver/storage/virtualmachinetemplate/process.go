package virtualmachinetemplate

import (
	"context"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
	Process(tpl *v1alpha1.VirtualMachineTemplate) (*virtv1.VirtualMachine, string, *field.Error)
}
type ProcessREST struct {
	client    templateclient.Interface
	processor processor
}

func NewProcessREST(client templateclient.Interface) *ProcessREST {
	return &ProcessREST{
		client:    client,
		processor: template.GetDefaultProcessor(),
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

		processed, err := processTemplate(ctx, p.client, p.processor, req.Body, ns, id)
		if err != nil {
			r.Error(err)
			return
		}

		r.Object(http.StatusOK, processed)
	}), nil
}

func (p *ProcessREST) NewConnectOptions() (options runtime.Object, include bool, path string) {
	return nil, false, ""
}

func (p *ProcessREST) ConnectMethods() []string {
	return []string{http.MethodPost}
}

func processTemplate(
	ctx context.Context,
	client templateclient.Interface,
	processor processor,
	body io.Reader,
	ns string,
	id string,
) (*subresourcesv1alpha1.ProcessedVirtualMachineTemplate, error) {
	opts := &subresourcesv1alpha1.ProcessOptions{}
	if err := yaml.NewYAMLOrJSONDecoder(body, jsonBufferSize).Decode(opts); err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("error parsing ProcessOptions: %v", err))
	}

	tpl, err := client.TemplateV1alpha1().VirtualMachineTemplates(ns).Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("error getting VirtualMachineTemplate: %w", err))
	}

	tpl.Spec.Parameters, err = template.MergeParameters(tpl.Spec.Parameters, opts.Parameters)
	if err != nil {
		return nil, apierrors.NewConflict(schema.GroupResource{
			Group:    tpl.GroupVersionKind().Group,
			Resource: tpl.Kind,
		}, id, err)
	}

	vm, msg, pErr := processor.Process(tpl)
	if pErr != nil {
		return nil, apierrors.NewInvalid(tpl.GroupVersionKind().GroupKind(), id, field.ErrorList{pErr})
	}

	return &subresourcesv1alpha1.ProcessedVirtualMachineTemplate{
		TemplateRef: &corev1.ObjectReference{
			Namespace: ns,
			Name:      id,
		},
		VirtualMachine: vm,
		Message:        msg,
	}, nil
}
