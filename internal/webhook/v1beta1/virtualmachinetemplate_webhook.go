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

package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	templatev1beta1 "kubevirt.io/virt-template-api/core/v1beta1"
	"kubevirt.io/virt-template-engine/template"
)

// SetupVirtualMachineTemplateWebhookWithManager registers the webhook for VirtualMachineTemplate in the manager.
func SetupVirtualMachineTemplateWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&templatev1beta1.VirtualMachineTemplate{}).
		WithValidator(&VirtualMachineTemplateCustomValidator{}).
		Complete()
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
//nolint:lll
// +kubebuilder:webhook:path=/validate-template-kubevirt-io-v1beta1-virtualmachinetemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=create;update,versions=v1beta1,name=vvirtualmachinetemplate-v1beta1.kb.io,admissionReviewVersions=v1

// VirtualMachineTemplateCustomValidator struct is responsible for validating the VirtualMachineTemplate resource
// when it is created, updated, or deleted.
type VirtualMachineTemplateCustomValidator struct{}

var _ webhook.CustomValidator = &VirtualMachineTemplateCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachineTemplate.
func (v *VirtualMachineTemplateCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	virtualmachinetemplate, ok := obj.(*templatev1beta1.VirtualMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachineTemplate object but got %T", obj)
	}

	return ValidateTemplate(virtualmachinetemplate)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachineTemplate.
func (v *VirtualMachineTemplateCustomValidator) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	virtualmachinetemplate, ok := newObj.(*templatev1beta1.VirtualMachineTemplate)
	if !ok {
		return nil, fmt.Errorf("expected a VirtualMachineTemplate object for the newObj but got %T", newObj)
	}

	return ValidateTemplate(virtualmachinetemplate)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type VirtualMachineTemplate.
func (v *VirtualMachineTemplateCustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateTemplate validates a VirtualMachineTemplate's parameter references and processing.
func ValidateTemplate(tpl *templatev1beta1.VirtualMachineTemplate) (admission.Warnings, error) {
	warnings, errs := template.ValidateParameterReferences(tpl)
	if len(errs) > 0 {
		return warnings, errs.ToAggregate()
	}

	processingWarnings, err := ValidateProcessing(tpl)
	warnings = append(warnings, processingWarnings...)

	return warnings, err
}

// ValidateProcessing attempts to process the template to verify it produces
// a valid VirtualMachine definition. Only performs full processing validation when
// all required parameters have values (or generators), to avoid type mismatches from
// placeholder values.
func ValidateProcessing(tpl *templatev1beta1.VirtualMachineTemplate) ([]string, error) {
	var warnings []string
	for _, param := range tpl.Spec.Parameters {
		if param.Required && param.Value == "" && param.Generate == "" {
			warnings = append(warnings,
				fmt.Sprintf("processing validation skipped: required parameter %q has neither value nor generator", param.Name))
		}
	}
	if len(warnings) > 0 {
		return warnings, nil
	}

	if _, _, err := template.GetDefaultProcessor().Process(tpl); err != nil {
		return nil, fmt.Errorf("processing validation failed: %w", err)
	}

	return nil, nil
}
