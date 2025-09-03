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

package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	templatev1alpha1 "kubevirt.io/virt-template/api/v1alpha1"
)

// log is for logging in this package.
var virtualmachinetemplatelog = logf.Log.WithName("virtualmachinetemplate-resource")

// SetupVirtualMachineTemplateWebhookWithManager registers the webhook for VirtualMachineTemplate in the manager.
func SetupVirtualMachineTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&templatev1alpha1.VirtualMachineTemplate{}).
		WithValidator(&VirtualMachineTemplateCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
//nolint:lll
// +kubebuilder:webhook:path=/validate-template-kubevirt-io-v1alpha1-virtualmachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=create;update,versions=v1alpha1,name=vvirtualmachinetemplate-v1alpha1.kb.io,admissionReviewVersions=v1

// VirtualMachineTemplateCustomValidator struct is responsible for validating the VirtualMachineTemplate resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type VirtualMachineTemplateCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &VirtualMachineTemplateCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachineTemplate.
func (v *VirtualMachineTemplateCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	virtualmachinetemplate, ok := obj.(*templatev1alpha1.VirtualMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachineTemplate object but got %T", obj)
	}
	virtualmachinetemplatelog.Info("Validation for VirtualMachineTemplate upon creation", "name", virtualmachinetemplate.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachineTemplate.
func (v *VirtualMachineTemplateCustomValidator) ValidateUpdate(
	_ context.Context,
	oldObj,
	newObj runtime.Object,
) (admission.Warnings, error) {
	virtualmachinetemplate, ok := newObj.(*templatev1alpha1.VirtualMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachineTemplate object for the newObj but got %T", newObj)
	}
	virtualmachinetemplatelog.Info("Validation for VirtualMachineTemplate upon update", "name", virtualmachinetemplate.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachineTemplate.
func (v *VirtualMachineTemplateCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	virtualmachinetemplate, ok := obj.(*templatev1alpha1.VirtualMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachineTemplate object but got %T", obj)
	}
	virtualmachinetemplatelog.Info("Validation for VirtualMachineTemplate upon deletion", "name", virtualmachinetemplate.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
