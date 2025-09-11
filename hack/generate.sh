#!/bin/bash

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
_bin_dir="${_base_dir}/bin"
_client_staging_dir="${_base_dir}/staging/src/kubevirt.io/virt-template/client-go"
_out_dir="${_base_dir}/_out"

mkdir -p "${_out_dir}"

"${_bin_dir}/openapi-gen" \
    --output-dir "${_client_staging_dir}/api" \
    --output-pkg kubevirt.io/virt-template/client-go/api \
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
    kubevirt.io/virt-template/api/v1alpha1 \
    kubevirt.io/virt-template/api/subresourcesv1alpha1

"${_bin_dir}/client-gen" \
    --clientset-name template \
    --input-base kubevirt.io/virt-template \
    --input api/v1alpha1 \
    --output-dir "${_client_staging_dir}" \
    --output-pkg kubevirt.io/virt-template/client-go \
    --go-header-file "${_base_dir}/hack/boilerplate.go.txt"
