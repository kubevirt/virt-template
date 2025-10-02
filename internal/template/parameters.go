package template

import (
	"fmt"
	"slices"

	"kubevirt.io/virt-template/api/v1alpha1"
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
