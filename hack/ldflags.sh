#!/bin/bash

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

source "${_base_dir}/hack/version.sh"

KUBE_GIT_VERSION_FILE="${_base_dir}/_out/version" kube::version::ldflags
