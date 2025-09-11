#!/bin/bash

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

source "${_base_dir}/hack/version.sh"

kube::version::get_version_vars
kube::version::save_version_vars "${_base_dir}/_out/version"
