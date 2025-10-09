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

# Taken from https://github.com/kubevirt/cluster-network-addons-operator/blob/main/hack/init-buildx.sh

set -ex

check_buildx() {
  export DOCKER_CLI_EXPERIMENTAL=enabled

  if ! docker buildx > /dev/null 2>&1; then
     mkdir -p ~/.docker/cli-plugins
     BUILDX_VERSION=$(curl -s https://api.github.com/repos/docker/buildx/releases/latest | jq -r .tag_name)
     ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
     curl -L https://github.com/docker/buildx/releases/download/"${BUILDX_VERSION}"/buildx-"${BUILDX_VERSION}".linux-"${ARCH}" --output ~/.docker/cli-plugins/docker-buildx
     chmod a+x ~/.docker/cli-plugins/docker-buildx
  fi
}

create_or_use_buildx_builder() {
  local builder_name=$1
  if [ -z "$builder_name" ]; then
    echo "Error: Builder name is required."
    exit 1
  fi

  check_buildx

  current_builder="$(docker buildx inspect "${builder_name}" 2>/dev/null)" || echo "Builder '${builder_name}' not found"

  if ! grep -q "^Driver: docker$" <<<"${current_builder}" && \
     grep -q "linux/amd64" <<<"${current_builder}" && \
     grep -q "linux/arm64" <<<"${current_builder}" && \
     grep -q "linux/s390x" <<<"${current_builder}"; then
    echo "The current builder already has multi-architecture support (amd64, arm64, s390x)."
    echo "Skipping setup as the builder is already configured correctly."
    exit 0
  fi

  # Check if the builder already exists by parsing the output of `docker buildx ls`
  # We check if the builder_name appears in the list of active builders
  existing_builder=$(docker buildx ls | grep -w "$builder_name" | awk '{print $1}')

  if [ -n "$existing_builder" ]; then
    echo "Builder '$builder_name' already exists."
    echo "Using existing builder '$builder_name'."
    docker buildx use "$builder_name"
  else
    echo "Creating a new Docker Buildx builder: $builder_name"
    docker buildx create --driver-opt network=host --use --name "$builder_name"
    echo "The new builder '$builder_name' has been created and set as active."
  fi
}

if [ $# -eq 1 ]; then
  create_or_use_buildx_builder "$1"
else
  echo "Usage: $0 <builder_name>"
  echo "Example: $0 mybuilder"
  exit 1
fi
