#!/bin/bash

set -ex

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
_bin_dir="${_base_dir}/bin"

kubectl -n kubevirt wait deployment virt-template-controller --for condition=Available --timeout=5m
kubectl -n kubevirt wait deployment virt-template-apiserver --for condition=Available --timeout=5m
