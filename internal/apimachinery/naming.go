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

package apimachinery

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// Reserve 10 chars for the hash + 1 char for the separator ('-')
	hashLength             = 10
	maxGeneratedNameLength = validation.DNS1035LabelMaxLength - hashLength - 1
)

// GetStableName generates a deterministic name based on a base string and additional inputs.
func GetStableName(base string, inputs ...string) string {
	hash := computeHash(base, inputs)

	// Ensure it starts with a letter to satisfy DNS-1035
	if base == "" || !(base[0] >= 'a' && base[0] <= 'z') {
		base = "x-" + base
	}

	if len(base) > maxGeneratedNameLength {
		base = base[:maxGeneratedNameLength]
	}

	// Ensure we don't end with a hyphen after truncation
	base = strings.TrimRight(base, "-")

	name := fmt.Sprintf("%s-%s", base, hash)
	if errs := validation.IsDNS1035Label(name); len(errs) > 0 {
		// Fallback: If the base was garbage (e.g. "!!!"), just return the hash
		return fmt.Sprintf("obj-%s", hash)
	}

	return name
}

func computeHash(base string, inputs []string) string {
	hasher := sha256.New()
	hasher.Write([]byte(base))
	for _, input := range inputs {
		hasher.Write([]byte(input))
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))[:hashLength]
}
