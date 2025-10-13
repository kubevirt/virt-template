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

if [ "${VIRT_TEMPLATE_CRI}" == "" ]; then
    /bin/bash -c "$@"
    exit $?
fi

if [ "${VIRT_TEMPLATE_CRI}" != "docker" ] && [ "${VIRT_TEMPLATE_CRI}" != "podman" ]; then
    echo "VIRT_TEMPLATE_CRI must be set to either docker or podman"
    exit 1
fi

${VIRT_TEMPLATE_CRI} run -v "$(pwd):/virt-template:rw,Z" --rm "${IMG_BUILDER}:${IMG_BUILDER_TAG}" /usr/bin/bash -c "cd /virt-template && $*"
