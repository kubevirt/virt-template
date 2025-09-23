package fake

import (
	"context"

	"k8s.io/client-go/testing"

	templateapi "kubevirt.io/virt-template/api"
	"kubevirt.io/virt-template/api/subresourcesv1alpha1"
)

var virtualmachinetemplatesResource = subresourcesv1alpha1.GroupVersion.WithResource(templateapi.PluralResourceName)

func (f *fakeVirtualMachineTemplates) Process(_ context.Context, name string, options subresourcesv1alpha1.ProcessOptions) (*subresourcesv1alpha1.ProcessedVirtualMachineTemplate, error) {
	obj, err := f.Fake.Invokes(
		testing.NewCreateSubresourceAction(virtualmachinetemplatesResource, name, "process", f.Namespace(), &options),
		&subresourcesv1alpha1.ProcessedVirtualMachineTemplate{},
	)
	if obj == nil {
		return nil, err
	}
	return obj.(*subresourcesv1alpha1.ProcessedVirtualMachineTemplate), err
}
