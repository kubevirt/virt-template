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

if ! command -v yamllint &> /dev/null; then
  echo "yamllint is not installed, see https://github.com/adrienverge/yamllint#installation for more details."
  exit 1
fi

if ! command -v shellcheck &> /dev/null; then
  echo "shellcheck is not installed, see https://github.com/koalaman/shellcheck#installing for more details."
  exit 1
fi

if ! yamllint config; then
  exit 1
fi

if ! shellcheck hack/*.sh; then
  exit 1
fi
