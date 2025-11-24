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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	templateapi "kubevirt.io/virt-template-api/core"
	"kubevirt.io/virt-template-api/core/v1alpha1"
)

// VirtualMachineTemplateRequestReconciler reconciles a VirtualMachineTemplateRequest object
type VirtualMachineTemplateRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplaterequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplaterequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplaterequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VirtualMachineTemplateRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	tplReq := &v1alpha1.VirtualMachineTemplateRequest{}
	if err := r.Get(ctx, req.NamespacedName, tplReq); err != nil {
		log.Error(err, "unable to fetch VirtualMachineTemplateRequest")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		ObservedGeneration: tplReq.Generation,
		Reason:             "TemplateReady",
		Message:            "VirtualMachineTemplate was created successfully",
	})

	if err := r.Status().Update(ctx, tplReq); err != nil {
		log.Error(err, "unable to update VirtualMachineTemplateRequest status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineTemplateRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VirtualMachineTemplateRequest{}).
		Named(templateapi.SingularRequestResourceName).
		Complete(r)
}
