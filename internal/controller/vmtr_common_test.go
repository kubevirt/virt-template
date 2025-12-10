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

package controller_test

import (
	"context"

	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtv1 "kubevirt.io/api/core/v1"
	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"
	"kubevirt.io/client-go/kubecli"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/apimachinery"
)

// This file contains common constants and helpers shared among the vmtr_*_test.go files.

const (
	testVMName       = "test-vm"
	testVolumeName   = "test-volume"
	testDVName       = "test-dv"
	testSnapshotName = "test-volume-snapshot"
	testClaimName    = "test-claim"
	wrongUID         = "wrong-uid"
)

type fakeKubevirtClient struct {
	kubecli.KubevirtClient
	err error
}

type fakeExpandSpecInterface struct {
	kubecli.ExpandSpecInterface
	err error
}

func (f *fakeKubevirtClient) ExpandSpec(_ string) kubecli.ExpandSpecInterface {
	return &fakeExpandSpecInterface{err: f.err}
}

func (f *fakeExpandSpecInterface) ForVirtualMachine(vm *virtv1.VirtualMachine) (*virtv1.VirtualMachine, error) {
	if f.err != nil {
		return nil, f.err
	}
	return vm, nil
}

func createRequest(cli client.Client, testNamespace, testVMNamespace string) *v1alpha1.VirtualMachineTemplateRequest {
	tplReq := &v1alpha1.VirtualMachineTemplateRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-request-",
			Namespace:    testNamespace,
		},
		Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
			VirtualMachineRef: v1alpha1.VirtualMachineReference{
				Namespace: testVMNamespace,
				Name:      testVMName,
			},
		},
	}
	ExpectWithOffset(1, cli.Create(context.Background(), tplReq)).To(Succeed())
	return tplReq
}

func createSnapshot(
	cli client.Client,
	tplReq *v1alpha1.VirtualMachineTemplateRequest,
) *snapshotv1beta1.VirtualMachineSnapshot {
	snap := &snapshotv1beta1.VirtualMachineSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apimachinery.GetStableName(tplReq.Name, string(tplReq.UID)),
			Namespace: tplReq.Spec.VirtualMachineRef.Namespace,
			Labels: map[string]string{
				v1alpha1.LabelRequestUID: string(tplReq.UID),
			},
		},
	}
	ExpectWithOffset(1, cli.Create(context.Background(), snap)).To(Succeed())
	return snap
}

type snapshotStatusOpts struct {
	phase       snapshotv1beta1.VirtualMachineSnapshotPhase
	ready       bool
	progressing bool
}

type snapshotStatusOpt func(*snapshotStatusOpts)

func withPhase(phase snapshotv1beta1.VirtualMachineSnapshotPhase) snapshotStatusOpt {
	return func(opts *snapshotStatusOpts) {
		opts.phase = phase
	}
}

func withReady() snapshotStatusOpt {
	return func(opts *snapshotStatusOpts) {
		opts.ready = true
	}
}

func withProgressing() snapshotStatusOpt {
	return func(opts *snapshotStatusOpts) {
		opts.progressing = true
	}
}

func setSnapshotStatus(
	cli client.Client,
	snap *snapshotv1beta1.VirtualMachineSnapshot,
	optFns ...snapshotStatusOpt,
) *snapshotv1beta1.VirtualMachineSnapshot {
	opts := &snapshotStatusOpts{}
	for _, optFn := range optFns {
		optFn(opts)
	}

	readyStatus := corev1.ConditionFalse
	if opts.ready {
		readyStatus = corev1.ConditionTrue
	}
	progressingStatus := corev1.ConditionFalse
	if opts.progressing {
		progressingStatus = corev1.ConditionTrue
	}

	conditions := []snapshotv1beta1.Condition{
		{
			Type:   snapshotv1beta1.ConditionReady,
			Status: readyStatus,
		},
		{
			Type:   snapshotv1beta1.ConditionProgressing,
			Status: progressingStatus,
		},
	}

	snap.Status = &snapshotv1beta1.VirtualMachineSnapshotStatus{
		VirtualMachineSnapshotContentName: &snap.Name,
		Phase:                             opts.phase,
		Conditions:                        conditions,
	}
	ExpectWithOffset(1, cli.Status().Update(context.Background(), snap)).To(Succeed())
	return snap
}

func createSnapshotContent(
	cli client.Client,
	snap *snapshotv1beta1.VirtualMachineSnapshot,
) *snapshotv1beta1.VirtualMachineSnapshotContent {
	snapContent := &snapshotv1beta1.VirtualMachineSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snap.Name,
			Namespace: snap.Namespace,
		},
		Spec: snapshotv1beta1.VirtualMachineSnapshotContentSpec{
			VirtualMachineSnapshotName: &snap.Name,
			Source: snapshotv1beta1.SourceSpec{
				VirtualMachine: &snapshotv1beta1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testVMName,
						Namespace: snap.Namespace,
					},
					Spec: virtv1.VirtualMachineSpec{
						Template: &virtv1.VirtualMachineInstanceTemplateSpec{
							Spec: virtv1.VirtualMachineInstanceSpec{
								Volumes: []virtv1.Volume{
									{
										Name: testVolumeName,
										VolumeSource: virtv1.VolumeSource{
											PersistentVolumeClaim: &virtv1.PersistentVolumeClaimVolumeSource{
												PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
													ClaimName: testClaimName,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			VolumeBackups: []snapshotv1beta1.VolumeBackup{
				{
					VolumeName: testVolumeName,
					PersistentVolumeClaim: snapshotv1beta1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: testClaimName,
						},
					},
					VolumeSnapshotName: ptr.To(testSnapshotName),
				},
			},
		},
	}
	ExpectWithOffset(1, cli.Create(context.Background(), snapContent)).To(Succeed())
	return snapContent
}

func setSnapshotContentStatus(
	cli client.Client,
	snapContent *snapshotv1beta1.VirtualMachineSnapshotContent,
	ready bool,
) *snapshotv1beta1.VirtualMachineSnapshotContent {
	snapContent.Status = &snapshotv1beta1.VirtualMachineSnapshotContentStatus{
		ReadyToUse: &ready,
	}
	ExpectWithOffset(1, cli.Status().Update(context.Background(), snapContent)).To(Succeed())
	return snapContent
}

func createDataVolume(
	cli client.Client,
	tplReq *v1alpha1.VirtualMachineTemplateRequest,
) *cdiv1beta1.DataVolume {
	base := tplReq.Name
	if tplReq.Spec.TemplateName != "" {
		base = tplReq.Spec.TemplateName
	}
	name := apimachinery.GetStableName(base, string(tplReq.UID), testVolumeName)
	dv := &cdiv1beta1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tplReq.Namespace,
			Labels: map[string]string{
				v1alpha1.LabelRequestUID: string(tplReq.UID),
			},
		},
		Spec: cdiv1beta1.DataVolumeSpec{
			Source: &cdiv1beta1.DataVolumeSource{
				Snapshot: &cdiv1beta1.DataVolumeSourceSnapshot{
					Name:      name,
					Namespace: tplReq.Spec.VirtualMachineRef.Namespace,
				},
			},
			Storage: &cdiv1beta1.StorageSpec{},
		},
	}
	ExpectWithOffset(1, controllerutil.SetControllerReference(tplReq, dv, k8sClient.Scheme())).To(Succeed())
	ExpectWithOffset(1, cli.Create(context.Background(), dv)).To(Succeed())
	return dv
}

func setDataVolumeStatus(
	cli client.Client,
	dv *cdiv1beta1.DataVolume,
	phase cdiv1beta1.DataVolumePhase, ready, running bool,
) *cdiv1beta1.DataVolume {
	readyStatus := corev1.ConditionTrue
	if !ready {
		readyStatus = corev1.ConditionFalse
	}
	runningStatus := corev1.ConditionTrue
	if !running {
		runningStatus = corev1.ConditionFalse
	}
	dv.Status = cdiv1beta1.DataVolumeStatus{
		Phase: phase,
		Conditions: []cdiv1beta1.DataVolumeCondition{
			{
				Type:   cdiv1beta1.DataVolumeReady,
				Status: readyStatus,
			},
			{
				Type:   cdiv1beta1.DataVolumeRunning,
				Status: runningStatus,
			},
		},
	}
	ExpectWithOffset(1, cli.Status().Update(context.Background(), dv)).To(Succeed())
	return dv
}

func expectCondition(
	tplReq *v1alpha1.VirtualMachineTemplateRequest,
	conditionType string, status metav1.ConditionStatus, reason string,
	messageMatchers ...gomegatypes.GomegaMatcher,
) {
	cond := meta.FindStatusCondition(tplReq.Status.Conditions, conditionType)
	ExpectWithOffset(1, cond).ToNot(BeNil())
	// Satisfy linters, but already ensured above
	if cond != nil {
		ExpectWithOffset(1, cond.Status).To(Equal(status))
		ExpectWithOffset(1, cond.Reason).To(Equal(reason))
		for _, matcher := range messageMatchers {
			ExpectWithOffset(1, cond.Message).To(matcher)
		}
	}
}
