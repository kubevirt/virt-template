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
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtv1 "kubevirt.io/api/core/v1"
	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	templateapi "kubevirt.io/virt-template-api/core"
	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/apimachinery"
	"kubevirt.io/virt-template/internal/logs"
)

const (
	uidField                            = "metadata.uid"
	requeueAfterSnapshotContentNotReady = 10 * time.Second

	paramNameName   = "NAME"
	paramName       = "${" + paramNameName + "}"
	paramNameSuffix = "-" + paramName

	logNS              = "ns"
	logName            = "name"
	logSnapNS          = "snapNS"
	logSnapName        = "snapName"
	logSnapContentNS   = "snapContentNS"
	logSnapContentName = "snapContentName"
	logDVNS            = "dvNS"
	logDVName          = "dvName"
	logTplNS           = "tplNS"
	logTplName         = "tplName"
	logDVTName         = "dvtName"
	logVolName         = "volName"
	logStatus          = "status"
	logReason          = "reason"
	logMessage         = "message"
	logUID             = "uid"
	logCount           = "count"

	annImmediateBinding = "cdi.kubevirt.io/storage.bind.immediate.requested"
)

var snapshotErrorReasons = []string{
	"In error state",
	"Source does not exist",
}

// VirtualMachineTemplateRequestReconciler reconciles a VirtualMachineTemplateRequest object
type VirtualMachineTemplateRequestReconciler struct {
	client.Client
	VirtClient kubecli.KubevirtClient
	Scheme     *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=create
// +kubebuilder:rbac:groups="",resources=pods,verbs=create
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes/source,verbs=create
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplates,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplaterequests,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplaterequests/status,verbs=get;patch
// +kubebuilder:rbac:groups=template.kubevirt.io,resources=virtualmachinetemplaterequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=snapshot.kubevirt.io,resources=virtualmachinesnapshots,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=snapshot.kubevirt.io,resources=virtualmachinesnapshotcontents,verbs=get;list;watch
// +kubebuilder:rbac:groups=subresources.kubevirt.io,resources=expand-vm-spec,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VirtualMachineTemplateRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := logf.FromContext(ctx)

	tplReq := &v1alpha1.VirtualMachineTemplateRequest{}
	if err := r.Get(ctx, req.NamespacedName, tplReq); err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Error(err, "Unable to fetch VirtualMachineTemplateRequest")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !tplReq.DeletionTimestamp.IsZero() {
		log.Info("Handling deletion of VirtualMachineTemplateRequest")
		return ctrl.Result{}, r.handleDeletion(ctx, tplReq)
	}
	if err := r.addFinalizer(ctx, tplReq); err != nil {
		return ctrl.Result{}, err
	}

	if !shouldReconcile(tplReq) {
		log.V(logs.DebugLevel).Info("VirtualMachineTemplateRequest is no longer progressing, not reconciling")
		return ctrl.Result{}, nil
	}

	helper, err := patch.NewHelper(tplReq, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		setStatusConditions(ctx, tplReq, retErr)
		retErr = errors.Join(retErr, helper.Patch(ctx, tplReq))
	}()

	// Remove Progressing condition to reset state for this reconcile.
	// It will be set by setStatusConditions or manually for permanent failures.
	meta.RemoveStatusCondition(&tplReq.Status.Conditions, v1alpha1.ConditionProgressing)

	if validateErr := validateRequest(tplReq); validateErr != nil {
		log.Error(validateErr, "Error validating VirtualMachineTemplateRequest")
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonInvalidConfiguration, "%s", validateErr.Error())
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonInvalidConfiguration)
		return ctrl.Result{}, nil
	}

	tpl, err := r.getTemplate(ctx, tplReq)
	if err != nil {
		log.Error(err, "Error getting template")
		return ctrl.Result{}, err
	}
	if tpl == nil {
		var processResult *ctrl.Result
		tpl, processResult, err = r.processRequest(ctx, tplReq)
		if processResult != nil || err != nil {
			return ptr.Deref(processResult, ctrl.Result{}), err
		}
	}

	if err := r.setDataVolumeOwnerReferences(ctx, tplReq, tpl); err != nil {
		log.Error(err, "Error setting owner references")
		return ctrl.Result{}, err
	}

	if err := r.deleteSnapshot(ctx, tplReq); err != nil {
		return ctrl.Result{}, err
	}

	syncTemplateStatusConditions(ctx, tplReq, tpl)

	return ctrl.Result{}, nil
}

func (r *VirtualMachineTemplateRequestReconciler) addFinalizer(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
) error {
	if !controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup) {
		logf.FromContext(ctx).V(logs.TraceLevel).Info("Adding finalizer to VirtualMachineTemplateRequest")
		tplReqCopy := tplReq.DeepCopy()
		controllerutil.AddFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)
		if err := r.Patch(ctx, tplReq, client.MergeFrom(tplReqCopy)); err != nil {
			return err
		}
	}

	return nil
}

func (r *VirtualMachineTemplateRequestReconciler) handleDeletion(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
) error {
	if controllerutil.ContainsFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup) {
		logf.FromContext(ctx).V(logs.TraceLevel).Info("Finalizing VirtualMachineTemplateRequest")
		if validateRequest(tplReq) == nil {
			if err := r.deleteSnapshot(ctx, tplReq); err != nil {
				return err
			}
		}
		tplReqCopy := tplReq.DeepCopy()
		controllerutil.RemoveFinalizer(tplReq, v1alpha1.FinalizerSnapshotCleanup)
		if err := r.Patch(ctx, tplReq, client.MergeFrom(tplReqCopy)); err != nil {
			return err
		}
	}

	return nil
}

func (r *VirtualMachineTemplateRequestReconciler) getTemplate(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
) (*v1alpha1.VirtualMachineTemplate, error) {
	tpl := emptyTemplate(tplReq)
	if err := r.Get(ctx, client.ObjectKeyFromObject(tpl), tpl); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	if tpl.Labels[v1alpha1.LabelRequestUID] != string(tplReq.UID) {
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		return nil, fmt.Errorf("existing VirtualMachineTemplate %s/%s was not created by this request", tpl.Namespace, tpl.Name)
	}

	setTemplateRef(tplReq, tpl)

	return tpl, nil
}

func (r *VirtualMachineTemplateRequestReconciler) processRequest(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
) (*v1alpha1.VirtualMachineTemplate, *ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.V(logs.DebugLevel).Info("Processing VirtualMachineTemplateRequest")

	snap := emptySnapshot(tplReq)
	if err := r.Get(ctx, client.ObjectKeyFromObject(snap), snap); k8serrors.IsNotFound(err) {
		log.Info("Creating VirtualMachineSnapshot", logSnapNS, snap.Namespace, logSnapName, snap.Name)
		if snap, err = r.createSnapshot(ctx, tplReq); err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		log.Error(err, "Unable to fetch VirtualMachineSnapshot")
		return nil, nil, err
	}

	if snap.Labels[v1alpha1.LabelRequestUID] != string(tplReq.UID) {
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		return nil, nil, fmt.Errorf("virtualMachineSnapshot %s/%s does not belong to this request", snap.Namespace, snap.Name)
	}

	if isTrue, _ := isSnapshotStatusConditionTrue(snap, snapshotv1beta1.ConditionReady); !isTrue {
		syncSnapshotStatusConditions(ctx, tplReq, snap)
		return nil, &ctrl.Result{}, nil
	}

	snapContent, err := r.getSnapshotContent(ctx, tplReq, snap)
	if err != nil {
		log.Error(err, "Unable to fetch VirtualMachineSnapshotContent")
		return nil, nil, err
	}

	if ready, readyErr := isSnapshotContentReady(snapContent); readyErr != nil {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed, "%s", readyErr.Error())
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		return nil, nil, readyErr
	} else if !ready {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			"Waiting for VirtualMachineSnapshotContent %s/%s to be ready", snapContent.Namespace, snapContent.Name)
		setProgressingCondition(ctx, tplReq, metav1.ConditionTrue, v1alpha1.ReasonWaiting)
		return nil, &ctrl.Result{RequeueAfter: requeueAfterSnapshotContentNotReady}, nil
	}

	log.V(logs.DebugLevel).Info("Cloning VirtualMachineSnapshotContent",
		logSnapContentNS, snapContent.Namespace, logSnapContentName, snapContent.Name)
	if cloneErr := r.cloneSnapshotContent(ctx, tplReq, snapContent); cloneErr != nil {
		log.Error(cloneErr, "Unable to clone VirtualMachineSnapshotContent")
		return nil, nil, cloneErr
	}

	if ready, readyErr := r.isSnapshotContentCloneReady(ctx, tplReq, snapContent); !ready {
		log.V(logs.DebugLevel).Info("VirtualMachineSnapshotContent clone is not ready yet",
			logSnapContentNS, snapContent.Namespace, logSnapContentName, snapContent.Name)
		return nil, &ctrl.Result{}, readyErr
	}

	tpl, err := r.createTemplate(ctx, tplReq, snapContent)
	if err != nil {
		log.Error(err, "Failed to create VirtualMachineTemplate")
		return nil, nil, err
	}

	return tpl, nil, nil
}

func (r *VirtualMachineTemplateRequestReconciler) createSnapshot(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
) (*snapshotv1beta1.VirtualMachineSnapshot, error) {
	snap := newSnapshot(tplReq)
	if err := r.Create(ctx, snap); err != nil {
		return nil, err
	}

	return snap, nil
}

func (r *VirtualMachineTemplateRequestReconciler) getSnapshotContent(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	snap *snapshotv1beta1.VirtualMachineSnapshot,
) (*snapshotv1beta1.VirtualMachineSnapshotContent, error) {
	if snap.Status.VirtualMachineSnapshotContentName == nil || *snap.Status.VirtualMachineSnapshotContentName == "" {
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		return nil, fmt.Errorf("virtualMachineSnapshot %s/%s does not have a VirtualMachineSnapshotContentName", snap.Namespace, snap.Name)
	}

	snapContent := &snapshotv1beta1.VirtualMachineSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: snap.Namespace,
			Name:      *snap.Status.VirtualMachineSnapshotContentName,
		},
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(snapContent), snapContent); err != nil {
		return nil, err
	}

	return snapContent, nil
}

func (r *VirtualMachineTemplateRequestReconciler) cloneSnapshotContent(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	snapContent *snapshotv1beta1.VirtualMachineSnapshotContent,
) error {
	for _, vol := range snapContent.Spec.VolumeBackups {
		dv := emptyDv(tplReq.Namespace, getDvName(tplReq, vol.VolumeName))
		if err := r.Get(ctx, client.ObjectKeyFromObject(dv), dv); err != nil {
			if !k8serrors.IsNotFound(err) {
				return err
			}
		} else {
			if dv.Labels[v1alpha1.LabelRequestUID] != string(tplReq.UID) {
				setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
				return fmt.Errorf("dataVolume %s/%s does not belong to this request", dv.Namespace, dv.Name)
			}
			continue
		}

		dv = newDv(dv.Namespace, dv.Name, string(tplReq.UID), snapContent.Namespace, *vol.VolumeSnapshotName)
		logf.FromContext(ctx).Info("Creating DataVolume", logDVNS, dv.Namespace, logDVName, dv.Name)
		if err := ctrl.SetControllerReference(tplReq, dv, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, dv); err != nil {
			return err
		}
	}

	return nil
}

func (r *VirtualMachineTemplateRequestReconciler) isSnapshotContentCloneReady(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	snapContent *snapshotv1beta1.VirtualMachineSnapshotContent,
) (bool, error) {
	for _, vol := range snapContent.Spec.VolumeBackups {
		dv := emptyDv(tplReq.Namespace, getDvName(tplReq, vol.VolumeName))
		if err := r.Get(ctx, client.ObjectKeyFromObject(dv), dv); err != nil {
			return false, client.IgnoreNotFound(err)
		}

		if isTrue, _ := isDataVolumeStatusConditionTrue(dv, cdiv1beta1.DataVolumeReady); !isTrue {
			syncDataVolumeStatusConditions(ctx, tplReq, dv)
			return false, nil
		}
	}

	return true, nil
}

func (r *VirtualMachineTemplateRequestReconciler) deleteSnapshot(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
) error {
	snap := emptySnapshot(tplReq)
	logf.FromContext(ctx).V(logs.DebugLevel).Info("Deleting VirtualMachineSnapshot", logSnapNS, snap.Namespace, logSnapName, snap.Name)
	return client.IgnoreNotFound(r.Delete(ctx, snap))
}

func (r *VirtualMachineTemplateRequestReconciler) createTemplate(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	snapContent *snapshotv1beta1.VirtualMachineSnapshotContent,
) (*v1alpha1.VirtualMachineTemplate, error) {
	vm, err := r.getExpandedVM(ctx, tplReq, snapContent)
	if err != nil {
		return nil, err
	}

	if vm.Spec.Template == nil {
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		return nil, fmt.Errorf("source VirtualMachine %s/%s has no template spec", vm.Namespace, vm.Name)
	}

	for _, volBackup := range snapContent.Spec.VolumeBackups {
		dvName := transformVolume(ctx, &vm.Spec.Template.Spec.Volumes, volBackup.VolumeName)
		if dvName == "" {
			dvName = volBackup.VolumeName
		}
		transformOrAddDVT(ctx, &vm.Spec.DataVolumeTemplates, dvName, tplReq.Namespace, getDvName(tplReq, volBackup.VolumeName))
	}

	tpl := newTemplate(tplReq, &vm.Spec)
	logf.FromContext(ctx).Info("Creating VirtualMachineTemplate", logTplNS, tpl.Namespace, logTplName, tpl.Name)
	if err := r.Client.Create(ctx, tpl); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed, "%s", err.Error())
			setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		}
		return nil, err
	}

	setTemplateRef(tplReq, tpl)

	return tpl, nil
}

func (r *VirtualMachineTemplateRequestReconciler) getExpandedVM(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	snapContent *snapshotv1beta1.VirtualMachineSnapshotContent,
) (*virtv1.VirtualMachine, error) {
	if snapContent.Spec.Source.VirtualMachine == nil {
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
		return nil, fmt.Errorf("virtualMachineSnapshotContent %s/%s has no source VirtualMachine",
			snapContent.Namespace, snapContent.Name)
	}

	vm := &virtv1.VirtualMachine{
		ObjectMeta: snapContent.Spec.Source.VirtualMachine.ObjectMeta,
		Spec:       snapContent.Spec.Source.VirtualMachine.Spec,
		Status:     snapContent.Spec.Source.VirtualMachine.Status,
	}

	// By expanding the VM the instance types and preferences and their revisions are removed
	// from the VM's definition. This makes handling a lot easier since no ControllerRevisions need to be copied.
	// TODO: Add support for ControllerRevisions so instance types and preferences can be kept
	vm, err := r.VirtClient.ExpandSpec(snapContent.Namespace).ForVirtualMachine(vm)
	if err != nil {
		return nil, err
	}

	return vm, nil
}

func (r *VirtualMachineTemplateRequestReconciler) setDataVolumeOwnerReferences(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	tpl *v1alpha1.VirtualMachineTemplate,
) error {
	dvs := cdiv1beta1.DataVolumeList{}
	if err := r.List(ctx, &dvs, client.MatchingLabels{v1alpha1.LabelRequestUID: string(tplReq.UID)}); err != nil {
		return err
	}

	for _, dv := range dvs.Items {
		if metav1.IsControlledBy(&dv, tpl) {
			continue
		}

		logf.FromContext(ctx).V(logs.DebugLevel).Info("Setting owner references of DataVolume", logDVNS, dv.Namespace, logDVName, dv.Name)
		dvCopy := dv.DeepCopy()
		if err := controllerutil.RemoveControllerReference(tplReq, &dv, r.Scheme); err != nil {
			return err
		}
		if err := controllerutil.SetControllerReference(tpl, &dv, r.Scheme); err != nil {
			return err
		}
		if err := r.Patch(ctx, &dv, client.MergeFrom(dvCopy)); err != nil {
			return err
		}
	}

	return nil
}

func shouldReconcile(tplReq *v1alpha1.VirtualMachineTemplateRequest) bool {
	progressing := meta.FindStatusCondition(tplReq.Status.Conditions, v1alpha1.ConditionProgressing)

	// Do not restart on changes to the spec
	return progressing == nil ||
		progressing.Status != metav1.ConditionFalse
}

func validateRequest(tplReq *v1alpha1.VirtualMachineTemplateRequest) error {
	if tplReq.Spec.VirtualMachineRef.Namespace == "" {
		return errors.New("virtualMachineRef.namespace cannot be empty")
	}
	if tplReq.Spec.VirtualMachineRef.Name == "" {
		return errors.New("virtualMachineRef.name cannot be empty")
	}

	return nil
}

func setStatusConditions(ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest, retErr error) {
	if retErr != nil {
		logf.FromContext(ctx).Error(retErr, "Reconciliation failed")
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed, "%s", retErr.Error())
		if meta.FindStatusCondition(tplReq.Status.Conditions, v1alpha1.ConditionProgressing) == nil {
			setProgressingCondition(ctx, tplReq, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
		}

		return
	}

	progressingStatus := metav1.ConditionTrue
	progressingReason := v1alpha1.ReasonReconciling

	cond := meta.FindStatusCondition(tplReq.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonReconciling, "")
	} else if cond.Status == metav1.ConditionTrue {
		progressingStatus = metav1.ConditionFalse
		progressingReason = v1alpha1.ReasonReconciled
	}

	if meta.FindStatusCondition(tplReq.Status.Conditions, v1alpha1.ConditionProgressing) == nil {
		setProgressingCondition(ctx, tplReq, progressingStatus, progressingReason)
	}
}

func setReadyCondition(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	status metav1.ConditionStatus, reason, message string, messageArgs ...any,
) {
	formattedMsg := fmt.Sprintf(message, messageArgs...)
	logf.FromContext(ctx).V(logs.TraceLevel).Info("Setting Ready condition", logStatus, status, logReason, reason, logMessage, formattedMsg)
	meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ConditionReady,
		Status:             status,
		ObservedGeneration: tplReq.Generation,
		Reason:             reason,
		Message:            formattedMsg,
	})
}

func setProgressingCondition(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	status metav1.ConditionStatus, reason string,
) {
	const message = ""
	logf.FromContext(ctx).V(logs.TraceLevel).Info("Setting Progressing condition", logStatus, status, logReason, reason, logMessage, message)
	meta.SetStatusCondition(&tplReq.Status.Conditions, metav1.Condition{
		Type:               v1alpha1.ConditionProgressing,
		Status:             status,
		ObservedGeneration: tplReq.Generation,
		Reason:             reason,
		Message:            message,
	})
}

func setTemplateRef(
	tplReq *v1alpha1.VirtualMachineTemplateRequest,
	tpl *v1alpha1.VirtualMachineTemplate,
) {
	tplReq.Status.TemplateRef = &corev1.LocalObjectReference{Name: tpl.Name}
}

func syncSnapshotStatusConditions(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	snap *snapshotv1beta1.VirtualMachineSnapshot,
) {
	logf.FromContext(ctx).V(logs.DebugLevel).Info("Syncing status conditions from VirtualMachineSnapshot",
		logSnapNS, snap.Namespace, logSnapName, snap.Name)

	failed, _ := isSnapshotStatusConditionTrue(snap, snapshotv1beta1.ConditionFailure)
	if !failed && isSnapshotProgressing(snap) {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			"Waiting for VirtualMachineSnapshot %s/%s to be ready", snap.Namespace, snap.Name)
	} else {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed,
			"VirtualMachineSnapshot %s/%s failed", snap.Namespace, snap.Name)
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	}
}

func isSnapshotStatusConditionTrue(
	snap *snapshotv1beta1.VirtualMachineSnapshot, condType snapshotv1beta1.ConditionType,
) (isTrue, present bool) {
	cond := findSnapshotStatusCondition(snap, condType)
	if cond == nil {
		return false, false
	}

	return cond.Status == corev1.ConditionTrue, true
}

func findSnapshotStatusCondition(
	snap *snapshotv1beta1.VirtualMachineSnapshot, condType snapshotv1beta1.ConditionType,
) *snapshotv1beta1.Condition {
	if snap.Status == nil {
		return nil
	}

	for _, cond := range snap.Status.Conditions {
		if cond.Type == condType {
			return &cond
		}
	}

	return nil
}

func isSnapshotProgressing(snap *snapshotv1beta1.VirtualMachineSnapshot) bool {
	cond := findSnapshotStatusCondition(snap, snapshotv1beta1.ConditionProgressing)
	if cond == nil {
		return true
	}

	if cond.Status == corev1.ConditionTrue {
		return true
	}

	// Condition is false, but check if phase still indicates progress
	// and reason is not a known error.
	phaseInProgress := snap.Status.Phase == snapshotv1beta1.PhaseUnset ||
		snap.Status.Phase == snapshotv1beta1.InProgress
	hasErrorReason := slices.Contains(snapshotErrorReasons, cond.Reason)

	return phaseInProgress && !hasErrorReason
}

func syncDataVolumeStatusConditions(ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest, dv *cdiv1beta1.DataVolume) {
	logf.FromContext(ctx).V(logs.DebugLevel).Info(
		"Syncing status conditions from DataVolume", logDVNS, dv.Namespace, logDVName, dv.Name)

	progressing, present := isDataVolumeStatusConditionTrue(dv, cdiv1beta1.DataVolumeRunning)

	// Fallback to status.phase, DataVolume can be progressing but status condition is false
	if !progressing && present {
		if dv.Status.Phase != cdiv1beta1.Failed {
			progressing = true
		}
	}

	if progressing || !present {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			"Waiting for DataVolume %s/%s to be ready", dv.Namespace, dv.Name)
		setProgressingCondition(ctx, tplReq, metav1.ConditionTrue, v1alpha1.ReasonWaiting)
	} else {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed, "DataVolume %s/%s failed", dv.Namespace, dv.Name)
		setProgressingCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	}
}

func isDataVolumeStatusConditionTrue(
	dv *cdiv1beta1.DataVolume, condType cdiv1beta1.DataVolumeConditionType,
) (isTrue, present bool) {
	for _, condition := range dv.Status.Conditions {
		if condition.Type == condType {
			return condition.Status == corev1.ConditionTrue, true
		}
	}

	return false, false
}

func syncTemplateStatusConditions(
	ctx context.Context, tplReq *v1alpha1.VirtualMachineTemplateRequest,
	tpl *v1alpha1.VirtualMachineTemplate,
) {
	logf.FromContext(ctx).V(logs.DebugLevel).Info("Syncing status conditions from VirtualMachineTemplate",
		logTplNS, tpl.Namespace, logTplName, tpl.Name)
	if cond := meta.FindStatusCondition(tpl.Status.Conditions, v1alpha1.ConditionReady); cond == nil || cond.Status != metav1.ConditionTrue {
		setReadyCondition(ctx, tplReq, metav1.ConditionFalse, v1alpha1.ReasonWaiting,
			"Waiting for VirtualMachineTemplate %s/%s to be ready", tpl.Namespace, tpl.Name)
	} else {
		setReadyCondition(ctx, tplReq, metav1.ConditionTrue, v1alpha1.ReasonReconciled, "%s", cond.Message)
	}
}

func isSnapshotContentReady(snapContent *snapshotv1beta1.VirtualMachineSnapshotContent) (bool, error) {
	if snapContent.Status == nil || snapContent.Status.ReadyToUse == nil || !*snapContent.Status.ReadyToUse {
		return false, nil
	}

	for _, vol := range snapContent.Spec.VolumeBackups {
		if vol.VolumeSnapshotName == nil || *vol.VolumeSnapshotName == "" {
			return false, fmt.Errorf("virtualMachineSnapshotContent %s/%s is missing a VolumeSnapshotName for volume %s",
				snapContent.Namespace, snapContent.Name, vol.VolumeName)
		}
	}

	return true, nil
}

func newSnapshot(tplReq *v1alpha1.VirtualMachineTemplateRequest) *snapshotv1beta1.VirtualMachineSnapshot {
	snap := emptySnapshot(tplReq)
	snap.ObjectMeta.Labels = map[string]string{
		v1alpha1.LabelRequestUID: string(tplReq.UID),
	}
	snap.Spec = snapshotv1beta1.VirtualMachineSnapshotSpec{
		Source: corev1.TypedLocalObjectReference{
			APIGroup: &virtv1.VirtualMachineGroupVersionKind.Group,
			Kind:     virtv1.VirtualMachineGroupVersionKind.Kind,
			Name:     tplReq.Spec.VirtualMachineRef.Name,
		},
	}

	return snap
}

func emptySnapshot(tplReq *v1alpha1.VirtualMachineTemplateRequest) *snapshotv1beta1.VirtualMachineSnapshot {
	return &snapshotv1beta1.VirtualMachineSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tplReq.Spec.VirtualMachineRef.Namespace,
			Name:      apimachinery.GetStableName(tplReq.Name, string(tplReq.UID)),
		},
	}
}

func newDv(dvNamespace, dvName, tplReqUID, snapNamespace, snapName string) *cdiv1beta1.DataVolume {
	dv := emptyDv(dvNamespace, dvName)
	dv.Annotations = map[string]string{
		annImmediateBinding: "",
	}
	dv.Labels = map[string]string{
		v1alpha1.LabelRequestUID: tplReqUID,
	}
	dv.Spec = cdiv1beta1.DataVolumeSpec{
		Source: &cdiv1beta1.DataVolumeSource{
			Snapshot: &cdiv1beta1.DataVolumeSourceSnapshot{
				Namespace: snapNamespace,
				Name:      snapName,
			},
		},
		Storage: &cdiv1beta1.StorageSpec{},
	}

	return dv
}

func emptyDv(namespace, name string) *cdiv1beta1.DataVolume {
	return &cdiv1beta1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func newTemplate(tplReq *v1alpha1.VirtualMachineTemplateRequest, vmSpec *virtv1.VirtualMachineSpec) *v1alpha1.VirtualMachineTemplate {
	tpl := emptyTemplate(tplReq)
	tpl.Labels = map[string]string{
		v1alpha1.LabelRequestUID: string(tplReq.UID),
	}
	tpl.Spec = v1alpha1.VirtualMachineTemplateSpec{
		VirtualMachine: &runtime.RawExtension{
			Object: &virtv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: paramName,
				},
				Spec: *vmSpec,
			},
		},
		Parameters: []v1alpha1.Parameter{
			{
				Name:     paramNameName,
				Required: true,
			},
		},
	}

	return tpl
}

func emptyTemplate(tplReq *v1alpha1.VirtualMachineTemplateRequest) *v1alpha1.VirtualMachineTemplate {
	return &v1alpha1.VirtualMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getTemplateName(tplReq),
			Namespace: tplReq.Namespace,
		},
	}
}

func transformVolume(ctx context.Context, volumes *[]virtv1.Volume, volName string) string {
	for i := range *volumes {
		vol := &(*volumes)[i]

		if vol.Name != volName {
			continue
		}

		logf.FromContext(ctx).V(logs.TraceLevel).Info("Transforming Volume", logVolName, volName)

		dvName := volName
		if vol.DataVolume != nil {
			dvName = vol.DataVolume.Name
		}

		*vol = virtv1.Volume{
			Name: volName,
			VolumeSource: virtv1.VolumeSource{
				DataVolume: &virtv1.DataVolumeSource{
					Name: dvName + paramNameSuffix,
				},
			},
		}

		return dvName
	}

	return ""
}

func transformOrAddDVT(ctx context.Context, dvts *[]virtv1.DataVolumeTemplateSpec, volName, pvcNamespace, pvcName string) {
	log := logf.FromContext(ctx)

	dvtSpec := virtv1.DataVolumeTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: volName + paramNameSuffix,
		},
		Spec: cdiv1beta1.DataVolumeSpec{
			Source: &cdiv1beta1.DataVolumeSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Namespace: pvcNamespace,
					Name:      pvcName,
				},
			},
			Storage: &cdiv1beta1.StorageSpec{},
		},
	}

	for i := range *dvts {
		dvt := &(*dvts)[i]
		if dvt.Name == volName {
			log.V(logs.TraceLevel).Info("Transforming DataVolumeTemplate", logDVTName, volName)
			*dvt = dvtSpec
			return
		}
	}

	log.V(logs.TraceLevel).Info("Adding DataVolumeTemplate", logDVTName, volName)
	*dvts = append(*dvts, dvtSpec)
}

func getTemplateName(tplReq *v1alpha1.VirtualMachineTemplateRequest) string {
	name := tplReq.Name
	if tplReq.Spec.TemplateName != "" {
		name = tplReq.Spec.TemplateName
	}
	return name
}

func getDvName(tplReq *v1alpha1.VirtualMachineTemplateRequest, volumeName string) string {
	return apimachinery.GetStableName(getTemplateName(tplReq), string(tplReq.UID), volumeName)
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineTemplateRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Add indexer required for enqueueRequestByUID
	err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.VirtualMachineTemplateRequest{}, uidField,
		func(obj client.Object) []string {
			return []string{string(obj.GetUID())}
		},
	)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VirtualMachineTemplateRequest{}).
		Named(templateapi.SingularRequestResourceName).
		Watches(&v1alpha1.VirtualMachineTemplate{}, handler.EnqueueRequestsFromMapFunc(r.EnqueueRequestByUID)).
		Watches(&snapshotv1beta1.VirtualMachineSnapshot{}, handler.EnqueueRequestsFromMapFunc(r.EnqueueRequestByUID)).
		Owns(&cdiv1beta1.DataVolume{}).
		Complete(r)
}

func (r *VirtualMachineTemplateRequestReconciler) EnqueueRequestByUID(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)

	requestUID, ok := obj.GetLabels()[v1alpha1.LabelRequestUID]
	if !ok {
		return nil
	}

	list := &v1alpha1.VirtualMachineTemplateRequestList{}
	if err := r.List(ctx, list, client.MatchingFields{uidField: requestUID}); err != nil {
		log.Error(err, "Unable to list VirtualMachineTemplateRequests")
		return nil
	}
	if count := len(list.Items); count != 1 {
		if count > 1 {
			log.V(logs.DebugLevel).Info("Expected exactly one VirtualMachineTemplateRequest", logUID, requestUID, logCount, count)
		}
		return nil
	}

	namespace := list.Items[0].Namespace
	name := list.Items[0].Name
	log.V(logs.TraceLevel).Info("Enqueueing VirtualMachineTemplateRequest", logNS, namespace, logName, name)
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}}
}
