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

if [ -z "${BUILD_ARCH}" ] || [ -z "${PLATFORMS}" ] || [ -z "${IMG}" ] || [ -z "${CONTAINERFILE}" ] || [ -z "${DOCKER_BUILDER}" ]; then
  echo "Error: BUILD_ARCH, PLATFORMS, IMG, CONTAINERFILE and DOCKER_BUILDER must be set."
  exit 1
fi

IFS=',' read -r -a PLATFORM_LIST <<< "${PLATFORMS}"

BUILD_ARGS=(--build-arg BUILD_ARCH="${BUILD_ARCH}" -f "${CONTAINERFILE}" -t "${IMG}" --push .)

if [ ${#PLATFORM_LIST[@]} -eq 1 ]; then
  docker build --platform "${PLATFORMS}" "${BUILD_ARGS[@]}"
else
  ./hack/init-buildx.sh "${DOCKER_BUILDER}"
  docker buildx build --platform "${PLATFORMS}" "${BUILD_ARGS[@]}"
  docker buildx rm "${DOCKER_BUILDER}" 2>/dev/null || echo "Builder ${DOCKER_BUILDER} not found or already removed, skipping."
fi
