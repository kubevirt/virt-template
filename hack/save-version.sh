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
_out_dir="${_base_dir}/_out"

mkdir -p "${_out_dir}"

source "${_base_dir}/hack/version.sh"

kube::version::get_version_vars
kube::version::save_version_vars "${_out_dir}/version"
