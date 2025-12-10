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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtv1 "kubevirt.io/api/core/v1"
	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/apimachinery"
	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest controller DataVolume handling", func() {
	var (
		reconciler  *controller.VirtualMachineTemplateRequestReconciler
		tplReq      *v1alpha1.VirtualMachineTemplateRequest
		snapContent *snapshotv1beta1.VirtualMachineSnapshotContent
	)

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{},
			Scheme:     k8sClient.Scheme(),
		}

		tplReq = createRequest(k8sClient, testNamespace, testVMNamespace)
		snap := createSnapshot(k8sClient, tplReq)
		snap = setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
		snapContent = createSnapshotContent(k8sClient, snap)
		snapContent = setSnapshotContentStatus(k8sClient, snapContent, true)
	})

	It("should create DataVolume for single volume backup", func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		dv := &cdiv1beta1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apimachinery.GetStableName(tplReq.Name, string(tplReq.UID), testVolumeName),
				Namespace: testNamespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(dv), dv)).To(Succeed())

		verifyDV(tplReq, dv, testSnapshotName)
	})

	It("should create DataVolumes for multiple volume backups", func() {
		const (
			secondVolumeName   = "second-volume"
			secondSnapshotName = "second-volume-snapshot"
			secondClaimName    = "second-claim"
		)

		snapContent.Spec.Source.VirtualMachine.Spec.Template.Spec.Volumes = append(
			snapContent.Spec.Source.VirtualMachine.Spec.Template.Spec.Volumes,
			virtv1.Volume{
				Name: secondVolumeName,
				VolumeSource: virtv1.VolumeSource{
					PersistentVolumeClaim: &virtv1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: secondClaimName,
						},
					},
				},
			},
		)
		snapContent.Spec.VolumeBackups = append(
			snapContent.Spec.VolumeBackups,
			snapshotv1beta1.VolumeBackup{
				VolumeName: secondVolumeName,
				PersistentVolumeClaim: snapshotv1beta1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: secondClaimName,
					},
				},
				VolumeSnapshotName: ptr.To(secondSnapshotName),
			},
		)
		Expect(k8sClient.Update(context.Background(), snapContent)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		dv1 := &cdiv1beta1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apimachinery.GetStableName(tplReq.Name, string(tplReq.UID), testVolumeName),
				Namespace: testNamespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(dv1), dv1)).To(Succeed())
		verifyDV(tplReq, dv1, testSnapshotName)

		dv2 := &cdiv1beta1.DataVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apimachinery.GetStableName(tplReq.Name, string(tplReq.UID), secondVolumeName),
				Namespace: testNamespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(dv2), dv2)).To(Succeed())
		verifyDV(tplReq, dv2, secondSnapshotName)
	})

	It("should not recreate DataVolume if it already exists with correct UID", func() {
		dv := createDataVolume(k8sClient, tplReq)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		newDV := &cdiv1beta1.DataVolume{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(dv), newDV)).To(Succeed())

		Expect(newDV.UID).To(Equal(dv.UID))
	})

	It("should fail when DataVolume exists with wrong UID", func() {
		dv := createDataVolume(k8sClient, tplReq)
		dv.Labels[v1alpha1.LabelRequestUID] = wrongUID
		Expect(k8sClient.Update(context.Background(), dv)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := MatchRegexp("dataVolume .* does not belong to this request")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	})
})

func verifyDV(
	tplReq *v1alpha1.VirtualMachineTemplateRequest,
	dv *cdiv1beta1.DataVolume, snapshotName string,
) {
	Expect(dv.Annotations).To(HaveKey("cdi.kubevirt.io/storage.bind.immediate.requested"))
	Expect(dv.Labels[v1alpha1.LabelRequestUID]).To(Equal(string(tplReq.UID)))
	Expect(dv.Spec.Source.Snapshot).ToNot(BeNil())
	Expect(dv.Spec.Source.Snapshot.Name).To(Equal(snapshotName))
	Expect(dv.Spec.Source.Snapshot.Namespace).To(Equal(testVMNamespace))
	Expect(*dv.Spec.Storage).To(Equal(cdiv1beta1.StorageSpec{}))
	Expect(metav1.IsControlledBy(dv, tplReq)).To(BeTrue())
}
