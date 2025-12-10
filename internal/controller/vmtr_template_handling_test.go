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
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	virtv1 "kubevirt.io/api/core/v1"
	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	"kubevirt.io/virt-template-api/core/v1alpha1"

	"kubevirt.io/virt-template/internal/apimachinery"
	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest Controller VirtualMachineTemplate handling", func() {
	const (
		paramNameName   = "NAME"
		paramName       = "${" + paramNameName + "}"
		paramNameSuffix = "-" + paramName
	)

	var reconciler *controller.VirtualMachineTemplateRequestReconciler

	BeforeEach(func() {
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{},
			Scheme:     k8sClient.Scheme(),
		}
	})

	It("should fail when VirtualMachineSnapshotContent has no source VirtualMachine", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)
		snap := createSnapshot(k8sClient, tplReq)
		snap = setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
		snapContent := createSnapshotContent(k8sClient, snap)
		snapContent.Spec.Source.VirtualMachine = nil
		Expect(k8sClient.Update(context.Background(), snapContent)).To(Succeed())
		setSnapshotContentStatus(k8sClient, snapContent, true)
		dv := createDataVolume(k8sClient, tplReq)
		setDataVolumeStatus(k8sClient, dv, cdiv1beta1.Succeeded, true, false)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := MatchRegexp("virtualMachineSnapshotContent .* has no source VirtualMachine")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	})

	It("should fail when ExpandSpec returns an error", func() {
		msg := "failed to expand VM spec"
		reconciler.VirtClient = &fakeKubevirtClient{err: errors.New(msg)}

		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)
		snap := createSnapshot(k8sClient, tplReq)
		snap = setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
		snapContent := createSnapshotContent(k8sClient, snap)
		setSnapshotContentStatus(k8sClient, snapContent, true)
		dv := createDataVolume(k8sClient, tplReq)
		setDataVolumeStatus(k8sClient, dv, cdiv1beta1.Succeeded, true, false)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).To(MatchError(msg))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, Equal(msg))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)
	})

	It("should use request name when TemplateName is not specified", func() {
		p := setupTestPipeline(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.TplReq.Name,
				Namespace: p.TplReq.Namespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tpl), tpl)).To(Succeed())
	})

	It("should use TemplateName when specified", func() {
		const templateName = "my-custom-template"
		tplReq := &v1alpha1.VirtualMachineTemplateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-custom-name",
				Namespace: testNamespace,
			},
			Spec: v1alpha1.VirtualMachineTemplateRequestSpec{
				VirtualMachineRef: v1alpha1.VirtualMachineReference{
					Namespace: testVMNamespace,
					Name:      testVMName,
				},
				TemplateName: templateName,
			},
		}
		Expect(k8sClient.Create(context.Background(), tplReq)).To(Succeed())
		snap := createSnapshot(k8sClient, tplReq)
		snap = setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
		snapContent := createSnapshotContent(k8sClient, snap)
		setSnapshotContentStatus(k8sClient, snapContent, true)
		dv := createDataVolume(k8sClient, tplReq)
		setDataVolumeStatus(k8sClient, dv, cdiv1beta1.Succeeded, true, false)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      templateName,
				Namespace: tplReq.Namespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tpl), tpl)).To(Succeed())
	})

	It("should create template with correct VM spec transformation", func() {
		p := setupTestPipeline(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.TplReq.Name,
				Namespace: p.TplReq.Namespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tpl), tpl)).To(Succeed())

		Expect(tpl.Labels).To(HaveKeyWithValue(v1alpha1.LabelRequestUID, string(p.TplReq.UID)))
		Expect(tpl.Spec.Parameters).To(ContainElement(
			v1alpha1.Parameter{Name: paramNameName, Required: true},
		))

		vm := decodeVM(tpl.Spec.VirtualMachine.Raw)
		Expect(vm.Name).To(Equal(paramName))

		Expect(vm.Spec.Template.Spec.Volumes).To(HaveLen(1))
		Expect(vm.Spec.Template.Spec.Volumes[0].Name).To(Equal(testVolumeName))
		Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume).ToNot(BeNil())
		Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(Equal(testVolumeName + paramNameSuffix))

		Expect(vm.Spec.DataVolumeTemplates).To(HaveLen(1))
		Expect(vm.Spec.DataVolumeTemplates[0].Name).To(Equal(testVolumeName + paramNameSuffix))
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source).ToNot(BeNil())
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC).ToNot(BeNil())
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Namespace).To(Equal(p.TplReq.Namespace))
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Name).
			To(Equal(apimachinery.GetStableName(p.TplReq.Name, string(p.TplReq.UID), testVolumeName)))
	})

	It("should transform existing DataVolumeTemplate instead of adding new one", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)
		snap := createSnapshot(k8sClient, tplReq)
		snap = setSnapshotStatus(k8sClient, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
		snapContent := createSnapshotContent(k8sClient, snap)

		// Use separate names for Volume and DataVolume to verify correct matching
		// The Volume references the DataVolume by name via DataVolume.Name field
		snapContent.Spec.Source.VirtualMachine.Spec.DataVolumeTemplates = []virtv1.DataVolumeTemplateSpec{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: testDVName,
				},
				Spec: cdiv1beta1.DataVolumeSpec{
					Source: &cdiv1beta1.DataVolumeSource{
						Blank: &cdiv1beta1.DataVolumeBlankImage{},
					},
				},
			},
		}
		snapContent.Spec.Source.VirtualMachine.Spec.Template.Spec.Volumes = []virtv1.Volume{
			{
				Name: testVolumeName,
				VolumeSource: virtv1.VolumeSource{
					DataVolume: &virtv1.DataVolumeSource{
						Name: testDVName,
					},
				},
			},
		}
		Expect(k8sClient.Update(context.Background(), snapContent)).To(Succeed())

		setSnapshotContentStatus(k8sClient, snapContent, true)
		dv := createDataVolume(k8sClient, tplReq)
		setDataVolumeStatus(k8sClient, dv, cdiv1beta1.Succeeded, true, false)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		tpl := &v1alpha1.VirtualMachineTemplate{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tpl)).To(Succeed())

		vm := decodeVM(tpl.Spec.VirtualMachine.Raw)

		// Volume name should remain unchanged
		Expect(vm.Spec.Template.Spec.Volumes).To(HaveLen(1))
		Expect(vm.Spec.Template.Spec.Volumes[0].Name).To(Equal(testVolumeName))
		// Volume's DataVolume reference should be parameterized
		Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume).ToNot(BeNil())
		Expect(vm.Spec.Template.Spec.Volumes[0].DataVolume.Name).To(Equal(testDVName + paramNameSuffix))

		// DataVolumeTemplate should be transformed with parameterized name and PVC source
		Expect(vm.Spec.DataVolumeTemplates).To(HaveLen(1))
		Expect(vm.Spec.DataVolumeTemplates[0].Name).To(Equal(testDVName + paramNameSuffix))
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.Blank).To(BeNil())
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC).ToNot(BeNil())
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Namespace).To(Equal(tplReq.Namespace))
		Expect(vm.Spec.DataVolumeTemplates[0].Spec.Source.PVC.Name).
			To(Equal(apimachinery.GetStableName(tplReq.Name, string(tplReq.UID), testVolumeName)))
	})

	It("should delete snapshot after template is created", func() {
		p := setupTestPipeline(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		tpl := &v1alpha1.VirtualMachineTemplate{}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(p.TplReq), tpl)).To(Succeed())

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(p.Snap), p.Snap)
		Expect(err).To(MatchError(k8serrors.IsNotFound, "k8serrors.IsNotFound"))
	})

	It("should fail when template exists with different UID", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tplReq.Name,
				Namespace: testNamespace,
				Labels: map[string]string{
					v1alpha1.LabelRequestUID: wrongUID,
				},
			},
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), tpl)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		matcher := MatchRegexp("existing VirtualMachineTemplate .* was not created by this request")
		Expect(err).To(MatchError(matcher))

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonFailed, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonFailed)
	})

	It("should transfer DataVolume ownership from request to template", func() {
		p := setupTestPipeline(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      p.TplReq.Name,
				Namespace: p.TplReq.Namespace,
			},
		}
		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tpl), tpl)).To(Succeed())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(p.DV), p.DV)).To(Succeed())
		Expect(metav1.IsControlledBy(p.DV, tpl)).To(BeTrue())
		Expect(metav1.IsControlledBy(p.DV, p.TplReq)).To(BeFalse())
	})

	It("should skip ownership transfer when template already owns DataVolume", func() {
		p := setupTestPipeline(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(p.DV), p.DV)).To(Succeed())
		dvResourceVersion := p.DV.ResourceVersion

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(p.DV), p.DV)).To(Succeed())
		Expect(p.DV.ResourceVersion).To(Equal(dvResourceVersion))
	})

	It("should set templateRef in status when template is created", func() {
		p := setupTestPipeline(k8sClient, testNamespace, testVMNamespace)

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(p.TplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(p.TplReq), p.TplReq)).To(Succeed())
		Expect(p.TplReq.Status.TemplateRef).ToNot(BeNil())
		Expect(p.TplReq.Status.TemplateRef.Name).To(Equal(p.TplReq.Name))
	})

	It("should set templateRef in status when template already exists", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tplReq.Name,
				Namespace: testNamespace,
				Labels: map[string]string{
					v1alpha1.LabelRequestUID: string(tplReq.UID),
				},
			},
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), tpl)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		Expect(tplReq.Status.TemplateRef).ToNot(BeNil())
		Expect(tplReq.Status.TemplateRef.Name).To(Equal(tpl.Name))
	})

	It("should sync conditions from existing template with matching UID", func() {
		tplReq := createRequest(k8sClient, testNamespace, testVMNamespace)

		tpl := &v1alpha1.VirtualMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tplReq.Name,
				Namespace: testNamespace,
				Labels: map[string]string{
					v1alpha1.LabelRequestUID: string(tplReq.UID),
				},
			},
			Spec: v1alpha1.VirtualMachineTemplateSpec{
				VirtualMachine: &runtime.RawExtension{
					Object: &virtv1.VirtualMachine{},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), tpl)).To(Succeed())

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		matcher := ContainSubstring("Waiting for VirtualMachineTemplate")
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)

		meta.SetStatusCondition(&tpl.Status.Conditions, metav1.Condition{
			Type:   v1alpha1.ConditionReady,
			Status: metav1.ConditionFalse,
			Reason: v1alpha1.ReasonReconciling,
		})
		Expect(k8sClient.Status().Update(context.Background(), tpl)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonWaiting, matcher)
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionTrue, v1alpha1.ReasonReconciling)

		const msg = "VirtualMachineTemplate is ready to be processed"
		meta.SetStatusCondition(&tpl.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ConditionReady,
			Status:  metav1.ConditionTrue,
			Reason:  v1alpha1.ReasonReconciled,
			Message: msg,
		})
		Expect(k8sClient.Status().Update(context.Background(), tpl)).To(Succeed())

		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(tplReq),
		})
		Expect(err).ToNot(HaveOccurred())

		Expect(k8sClient.Get(context.Background(), client.ObjectKeyFromObject(tplReq), tplReq)).To(Succeed())
		expectCondition(tplReq, v1alpha1.ConditionReady, metav1.ConditionTrue, v1alpha1.ReasonReconciled, Equal(msg))
		expectCondition(tplReq, v1alpha1.ConditionProgressing, metav1.ConditionFalse, v1alpha1.ReasonReconciled)
	})
})

type testPipeline struct {
	TplReq      *v1alpha1.VirtualMachineTemplateRequest
	Snap        *snapshotv1beta1.VirtualMachineSnapshot
	SnapContent *snapshotv1beta1.VirtualMachineSnapshotContent
	DV          *cdiv1beta1.DataVolume
}

func setupTestPipeline(cli client.Client, testNamespace, testVMNamespace string) *testPipeline {
	tplReq := createRequest(cli, testNamespace, testVMNamespace)
	snap := createSnapshot(cli, tplReq)
	snap = setSnapshotStatus(cli, snap, withPhase(snapshotv1beta1.Succeeded), withReady())
	snapContent := createSnapshotContent(cli, snap)
	snapContent = setSnapshotContentStatus(cli, snapContent, true)
	dv := createDataVolume(cli, tplReq)
	dv = setDataVolumeStatus(cli, dv, cdiv1beta1.Succeeded, true, false)
	return &testPipeline{
		TplReq:      tplReq,
		Snap:        snap,
		SnapContent: snapContent,
		DV:          dv,
	}
}

func decodeVM(raw []byte) *virtv1.VirtualMachine {
	var obj map[string]interface{}
	ExpectWithOffset(1, json.Unmarshal(raw, &obj)).To(Succeed())
	vm := &virtv1.VirtualMachine{}
	ExpectWithOffset(1, runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj, vm, true)).To(Succeed())
	return vm
}
