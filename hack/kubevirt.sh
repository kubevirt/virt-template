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

set -ex

export KUBEVIRT_MEMORY_SIZE="${KUBEVIRT_MEMORY_SIZE:-16G}"
export KUBEVIRT_STORAGE="${KUBEVIRT_STORAGE:-rook-ceph-default}"
export KUBEVIRT_TAG="${KUBEVIRT_TAG:-main}"

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
_kubevirt_dir="${_base_dir}/_kubevirt"
_kubectl="${_base_dir}/_kubevirt/kubevirtci/cluster-up/kubectl.sh"
_kubessh="${_base_dir}/_kubevirt/kubevirtci/cluster-up/ssh.sh"
_kubevirtcicli="${_base_dir}/_kubevirt/kubevirtci/cluster-up/cli.sh"
_action=$1
shift

function kubevirt::fetch_kubevirt() {
  if [[ ! -d ${_kubevirt_dir} ]]; then
    git clone --depth 1 --branch "${KUBEVIRT_TAG}" https://github.com/kubevirt/kubevirt.git "${_kubevirt_dir}"
  fi
}

function kubevirt::up() {
  make cluster-up -C "${_kubevirt_dir}" && make cluster-sync -C "${_kubevirt_dir}"
  KUBECONFIG=$(kubevirt::kubeconfig)
  export KUBECONFIG

  echo "waiting for kubevirt to become ready, this can take a few minutes..."
  ${_kubectl} -n kubevirt wait kv kubevirt --for condition=Available --timeout=15m
}

function kubevirt::down() {
  make cluster-down -C "${_kubevirt_dir}"
}

function kubevirt::kubeconfig() {
  "${_kubevirt_dir}/kubevirtci/cluster-up/kubeconfig.sh"
}

function kubevirt::registry() {
  port=$(${_kubevirtcicli} ports registry 2>/dev/null)
  echo "localhost:${port}"
}

kubevirt::fetch_kubevirt

case ${_action} in
  "up")
    kubevirt::up
    ;;
  "down")
    kubevirt::down
    ;;
  "kubeconfig")
    kubevirt::kubeconfig
    ;;
  "registry")
    kubevirt::registry
    ;;
  "ssh")
    ${_kubessh} "$@"
    ;;
  "kubectl")
    ${_kubectl} "$@"
    ;;
  *)
    echo "No command provided, known commands are 'up', 'down', 'kubeconfig', 'registry', 'ssh' and 'kubectl'"
    exit 1
    ;;
esac
