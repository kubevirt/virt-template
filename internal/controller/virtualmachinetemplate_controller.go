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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	templateapi "kubevirt.io/virt-template-api/core"
	"kubevirt.io/virt-template-api/core/v1alpha1"
)

// VirtualMachineTemplateReconciler reconciles a VirtualMachineTemplate object
type VirtualMachineTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/status,verbs=get;patch
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VirtualMachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tpl := &v1alpha1.VirtualMachineTemplate{}
	if err := r.Get(ctx, req.NamespacedName, tpl); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	helper, err := patch.NewHelper(tpl, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&tpl.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: tpl.Generation,
		Reason:             v1alpha1.ReasonReconciled,
		Message:            "VirtualMachineTemplate is ready to be processed",
	})

	return ctrl.Result{}, helper.Patch(ctx, tpl)
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VirtualMachineTemplate{}).
		Named(templateapi.SingularResourceName).
		Complete(r)
}
