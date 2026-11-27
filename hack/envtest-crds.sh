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

# Pulls the third-party CRDs envtest needs from the cluster in KUBECONFIG into
# config/crd/testing, keeping only what envtest uses.

set -euo pipefail

_out_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/config/crd/testing"

for crd in \
  datavolumes.cdi.kubevirt.io \
  virtualmachinesnapshotcontents.snapshot.kubevirt.io \
  virtualmachinesnapshots.snapshot.kubevirt.io; do
  plural=${crd%%.*}
  file="${_out_dir}/${crd#*.}_${plural}.yaml"
  {
    echo "# Pulled from a live cluster by hack/envtest-crds.sh, do not edit."
    kubectl get crd "${crd}" -o json |
      jq '{apiVersion, kind, metadata: {name: .metadata.name}, spec: (.spec | del(.conversion))}' |
      kubectl create --dry-run=client -o yaml -f -
  } > "${file}"
done
