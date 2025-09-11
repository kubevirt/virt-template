package virtualmachinetemplate

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	templateapi "kubevirt.io/virt-template/api"
	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
)

// DummyREST is required to satisfy the k8s.io/apiserver conventions.
// A subresource cannot be served without a storage for its parent resource.
type DummyREST struct{}

func NewDummyREST() *DummyREST {
	return &DummyREST{}
}

var (
	_ = rest.Storage(&DummyREST{})
	_ = rest.Scoper(&DummyREST{})
	_ = rest.SingularNameProvider(&DummyREST{})
)

func (r *DummyREST) New() runtime.Object {
	return &subresourcesv1alpha1.VirtualMachineTemplate{}
}

func (r *DummyREST) Destroy() {}

func (r *DummyREST) NamespaceScoped() bool { return true }

func (r *DummyREST) GetSingularName() string {
	return templateapi.SingularResourceName
}
