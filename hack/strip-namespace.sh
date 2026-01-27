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

FILE=$1
if [ -z "$FILE" ]; then
  echo "Error: Provide a filename."
  exit 1
fi

# - Find a line starting with 'kind: <OurPolicies>'
# - Continue processing lines until we hit 'spec:' or '---' (end of block)
# - Inside that range, delete any line matching 'namespace:'

KINDS="ValidatingAdmissionPolicy|ValidatingAdmissionPolicyBinding"
sed -i -E "/kind: ($KINDS)/,/^(spec:|---)/ {
  /^\s*namespace:/d
}" "$FILE"