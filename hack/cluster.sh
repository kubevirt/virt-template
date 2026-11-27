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
# Sourced by hack/kubevirtci.sh and hack/kubevirt.sh.

# Enable a KubeVirt feature gate, keeping the ones already enabled.
function cluster::enable_feature_gate() {
  local kubectl=$1
  local gate=$2
  local gates

  gates=$(${kubectl} get -n kubevirt kubevirt kubevirt -o jsonpath='{.spec.configuration.developerConfiguration.featureGates}')
  if [[ -z ${gates} ]]; then
    ${kubectl} patch -n kubevirt kubevirt kubevirt --type merge \
      -p '{"spec": {"configuration": {"developerConfiguration": {"featureGates": ["'"${gate}"'"]}}}}'
  elif [[ ${gates} != *"\"${gate}\""* ]]; then
    ${kubectl} patch -n kubevirt kubevirt kubevirt --type json \
      -p '[{"op": "add", "path": "/spec/configuration/developerConfiguration/featureGates/-", "value": "'"${gate}"'"}]'
  fi
}
