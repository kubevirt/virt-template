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

package template

import (
	"fmt"
	"slices"

	"kubevirt.io/virt-template-api/core/v1alpha1"
)

func MergeParameters(tplParams []v1alpha1.Parameter, params map[string]string) ([]v1alpha1.Parameter, error) {
	newTplParams := slices.Clone(tplParams)
	for k, v := range params {
		found := false
		for i := range newTplParams {
			if newTplParams[i].Name == k {
				newTplParams[i].Value = v
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("parameter %s not found in template", k)
		}
	}
	return newTplParams, nil
}
