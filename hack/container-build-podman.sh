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

if [ -z "${BUILD_ARCH}" ] || [ -z "${PLATFORMS}" ] || [ -z "${IMG}" ] || [ -z "${CONTAINERFILE}" ]; then
  echo "Error: BUILD_ARCH, PLATFORMS, IMG and CONTAINERFILE must be set."
  exit 1
fi

IFS=',' read -r -a PLATFORM_LIST <<< "${PLATFORMS}"

# Remove any existing manifest and image
podman manifest rm "${IMG}" 2>/dev/null || true
podman rmi "${IMG}" 2>/dev/null || true

podman manifest create "${IMG}"

for PLATFORM in "${PLATFORM_LIST[@]}"; do
  podman build \
    --build-arg BUILD_ARCH="${BUILD_ARCH}" \
    --platform "${PLATFORM}" \
    --manifest "${IMG}" \
    -f "${CONTAINERFILE}" \
    .
done
