#!/bin/bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright The KubeVirt Authors.
#

set -e

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
_bin_dir="${_base_dir}/bin"
_client_staging_dir="${_base_dir}/staging/src/kubevirt.io/virt-template-client-go"
_out_dir="${_base_dir}/_out"

mkdir -p "${_out_dir}"

"${_bin_dir}/openapi-gen" \
  --output-dir "${_client_staging_dir}/api" \
  --output-pkg kubevirt.io/virt-template-client-go/api \
  --output-file openapi_generated.go \
  --go-header-file "${_base_dir}/hack/boilerplate.go.txt" \
  --report-filename "${_out_dir}/api-rule-violations.list" \
  k8s.io/api/core/v1 \
  k8s.io/apimachinery/pkg/api/resource \
  k8s.io/apimachinery/pkg/apis/meta/v1 \
  k8s.io/apimachinery/pkg/runtime \
  k8s.io/apimachinery/pkg/util/intstr \
  k8s.io/apimachinery/pkg/version \
  kubevirt.io/api/core/v1 \
  kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1 \
  kubevirt.io/virt-template-api/core/v1alpha1 \
  kubevirt.io/virt-template-api/core/subresourcesv1alpha1

"${_bin_dir}/client-gen" \
  --clientset-name virttemplate \
  --input-base kubevirt.io/virt-template-api \
  --input core/v1alpha1 \
  --output-dir "${_client_staging_dir}" \
  --output-pkg kubevirt.io/virt-template-client-go \
  --go-header-file "${_base_dir}/hack/boilerplate.go.txt"
