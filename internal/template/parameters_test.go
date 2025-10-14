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

package template_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/virt-template-api/core/v1alpha1"
	"kubevirt.io/virt-template/internal/template"
)

var _ = Describe("MergeParameters", func() {
	const (
		param1Name       = "NAME"
		param1DefaultVal = "default-name"
		param1Val        = "test-vm"
		param2Name       = "PREFERENCE"
		param2DefaultVal = "default-preference"
		param2Val        = "fedora"
		paramUnknownName = "UNKNOWN"
		paramUnknownVal  = "something"
	)

	var tplParams []v1alpha1.Parameter

	BeforeEach(func() {
		tplParams = []v1alpha1.Parameter{
			{
				Name:  param1Name,
				Value: param1DefaultVal,
			},
			{
				Name:  param2Name,
				Value: param2DefaultVal,
			},
		}
	})

	It("should merge single parameter successfully", func() {
		params := map[string]string{
			param1Name: param1Val,
		}

		newTplParams, err := template.MergeParameters(tplParams, params)
		Expect(err).ToNot(HaveOccurred())
		Expect(newTplParams[0].Value).To(Equal(param1Val))
		Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
	})

	It("should merge multiple parameters successfully", func() {
		params := map[string]string{
			param1Name: param1Val,
			param2Name: param2Val,
		}

		newTplParams, err := template.MergeParameters(tplParams, params)
		Expect(err).ToNot(HaveOccurred())
		Expect(newTplParams[0].Value).To(Equal(param1Val))
		Expect(newTplParams[1].Value).To(Equal(param2Val))
	})

	It("should handle nil params", func() {
		newTplParams, err := template.MergeParameters(tplParams, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(newTplParams[0].Value).To(Equal(param1DefaultVal))
		Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
	})

	It("should handle empty params", func() {
		newTplParams, err := template.MergeParameters(tplParams, map[string]string{})
		Expect(err).ToNot(HaveOccurred())
		Expect(newTplParams[0].Value).To(Equal(param1DefaultVal))
		Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
	})

	It("should return error for single parameter not in template", func() {
		params := map[string]string{
			paramUnknownName: paramUnknownVal,
		}

		newTplParams, err := template.MergeParameters(tplParams, params)
		Expect(err).To(MatchError(fmt.Sprintf("parameter %s not found in template", paramUnknownName)))
		Expect(newTplParams).To(BeNil())
	})

	It("should return error when one of multiple params not in template", func() {
		params := map[string]string{
			param1Name:       param1Val,
			paramUnknownName: paramUnknownVal,
		}

		newTplParams, err := template.MergeParameters(tplParams, params)
		Expect(err).To(MatchError(fmt.Sprintf("parameter %s not found in template", paramUnknownName)))
		Expect(newTplParams).To(BeNil())
	})

	It("should handle empty template parameters", func() {
		tplParams = []v1alpha1.Parameter{}

		params := map[string]string{
			param1Name: param1Val,
		}

		newTplParams, err := template.MergeParameters(tplParams, params)
		Expect(err).To(MatchError(fmt.Sprintf("parameter %s not found in template", param1Name)))
		Expect(newTplParams).To(BeNil())
	})

	It("should handle parameter with empty value", func() {
		params := map[string]string{
			param1Name: "",
		}

		newTplParams, err := template.MergeParameters(tplParams, params)
		Expect(err).ToNot(HaveOccurred())
		Expect(newTplParams[0].Value).To(BeEmpty())
		Expect(newTplParams[1].Value).To(Equal(param2DefaultVal))
	})
})
