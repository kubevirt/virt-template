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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1beta1 "kubevirt.io/api/snapshot/v1beta1"

	virtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/virt-template-api/core/v1beta1"

	"kubevirt.io/virt-template/internal/controller"
)

var _ = Describe("VirtualMachineTemplateRequest Controller Instancetype and Preference parametrization", func() {
	const (
		paramInstancetypeName = "INSTANCETYPE"
		paramInstancetype     = "${" + paramInstancetypeName + "}"

		testInstancetypeName = "u1.small"
		testPreferenceName   = "fedora"
		testRevisionName     = "some-revision-abcde"

		namespacedInstancetypeKind = "VirtualMachineInstancetype"
		namespacedPreferenceKind   = "VirtualMachinePreference"

		testInferFromVolume = "rootdisk"
	)

	var (
		reconciler   *controller.VirtualMachineTemplateRequestReconciler
		expandCalled bool
	)

	BeforeEach(func() {
		expandCalled = false
		reconciler = &controller.VirtualMachineTemplateRequestReconciler{
			Client:     k8sClient,
			VirtClient: &fakeKubevirtClient{called: &expandCalled},
			Scheme:     k8sClient.Scheme(),
		}
	})

	DescribeTable(
		"keeps references when matchers are cluster-scoped and resolved",
		func(
			instancetype *virtv1.InstancetypeMatcher,
			preference *virtv1.PreferenceMatcher,
			expectKeepInstancetype bool,
			expectKeepPreference bool,
		) {
			tpl, vm := reconcileWithModifiedContent(k8sClient, reconciler, func(snapContent *snapshotv1beta1.VirtualMachineSnapshotContent) {
				snapContent.Spec.Source.VirtualMachine.Spec.Instancetype = instancetype
				snapContent.Spec.Source.VirtualMachine.Spec.Preference = preference
			})

			Expect(expandCalled).To(BeFalse())

			if expectKeepInstancetype {
				Expect(vm.Spec.Instancetype).ToNot(BeNil())
				Expect(vm.Spec.Instancetype.Name).To(Equal(paramInstancetype))
				Expect(vm.Spec.Instancetype.RevisionName).To(BeEmpty())
				Expect(tpl.Spec.Parameters).To(ContainElement(
					v1beta1.Parameter{Name: paramInstancetypeName, Value: testInstancetypeName},
				))
			} else {
				Expect(vm.Spec.Instancetype).To(BeNil())
				for _, p := range tpl.Spec.Parameters {
					Expect(p.Name).ToNot(Equal(paramInstancetypeName))
				}
			}

			if expectKeepPreference {
				Expect(vm.Spec.Preference).ToNot(BeNil())
				Expect(vm.Spec.Preference.Name).To(Equal(testPreferenceName))
				Expect(vm.Spec.Preference.RevisionName).To(BeEmpty())
			} else {
				Expect(vm.Spec.Preference).To(BeNil())
			}

			for _, p := range tpl.Spec.Parameters {
				Expect(p.Name).ToNot(Equal("PREFERENCE"))
			}
		},
		Entry(
			"both cluster-scoped instancetype and preference",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName},
			&virtv1.PreferenceMatcher{Name: testPreferenceName},
			true, true,
		),
		Entry(
			"solo cluster instancetype",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName},
			nil,
			true, false,
		),
		Entry(
			"both with explicit cluster kind",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName, Kind: "VirtualMachineClusterInstancetype"},
			&virtv1.PreferenceMatcher{Name: testPreferenceName, Kind: "VirtualMachineClusterPreference"},
			true, true,
		),
		Entry(
			"solo cluster preference",
			nil,
			&virtv1.PreferenceMatcher{Name: testPreferenceName},
			false, true,
		),
		Entry(
			"both cluster-scoped with pinned revision",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName, RevisionName: testRevisionName},
			&virtv1.PreferenceMatcher{Name: testPreferenceName, RevisionName: testRevisionName},
			true, true,
		),
	)

	DescribeTable(
		"expands both when references cannot be kept",
		func(instancetype *virtv1.InstancetypeMatcher, preference *virtv1.PreferenceMatcher) {
			tpl, _ := reconcileWithModifiedContent(k8sClient, reconciler, func(snapContent *snapshotv1beta1.VirtualMachineSnapshotContent) {
				snapContent.Spec.Source.VirtualMachine.Spec.Instancetype = instancetype
				snapContent.Spec.Source.VirtualMachine.Spec.Preference = preference
			})

			Expect(expandCalled).To(BeTrue())
			for _, p := range tpl.Spec.Parameters {
				Expect(p.Name).ToNot(Equal(paramInstancetypeName))
			}
		},
		Entry(
			"both namespace-scoped with revision",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName, Kind: namespacedInstancetypeKind, RevisionName: testRevisionName},
			&virtv1.PreferenceMatcher{Name: testPreferenceName, Kind: namespacedPreferenceKind, RevisionName: testRevisionName},
		),
		Entry(
			"namespace-scoped instancetype",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName, Kind: namespacedInstancetypeKind},
			&virtv1.PreferenceMatcher{Name: testPreferenceName},
		),
		Entry(
			"instancetype waiting on volume-based inference",
			&virtv1.InstancetypeMatcher{InferFromVolume: testInferFromVolume},
			nil,
		),
		Entry(
			"only one of a mixed pair qualifies",
			&virtv1.InstancetypeMatcher{Name: testInstancetypeName},
			&virtv1.PreferenceMatcher{Name: testPreferenceName, Kind: namespacedPreferenceKind},
		),
		Entry(
			"both nil",
			nil,
			nil,
		),
	)
})
