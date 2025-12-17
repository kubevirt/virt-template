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

set -euo pipefail

CLI_PLATFORMS="${CLI_PLATFORMS:-linux/amd64,linux/arm64,linux/s390x,darwin/amd64,darwin/arm64,windows/amd64}"
IMG_TAG="${IMG_TAG:-latest}"

BUILD_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
BUILD_ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"

for platform in ${CLI_PLATFORMS//,/ }; do
    os="${platform%/*}"
    arch="${platform#*/}"
    echo "Building virttemplatectl for ${os}/${arch}"
    CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build -ldflags="-s -w" -trimpath -o "bin/virttemplatectl-${IMG_TAG}-${os}-${arch}" cmd/virttemplatectl/main.go
done

ln -sf "virttemplatectl-${IMG_TAG}-${BUILD_OS}-${BUILD_ARCH}" bin/virttemplatectl
