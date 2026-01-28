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

readonly GITHUB_FQDN=github.com
TEMP_BASE=$(mktemp -d -t virt-template-publish.XXXXXX)
readonly TEMP_BASE
trap 'rm -rf "${TEMP_BASE}"' EXIT

# Validate required variables early
: "${BUILD_ID:?BUILD_ID must be set}"

TARGET_BRANCH="${TARGET_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"
TARGET_TAG="${TARGET_TAG:-}"

# if we are not on default branch and there is no tag, do nothing
if [ -z "${TARGET_TAG}" ] && [ "${TARGET_BRANCH}" != "main" ]; then
  echo "not on a tag and not on main branch, nothing to do."
  exit 0
fi

function prepare_repo() {
  local -r name=${1:?name required}
  local -r staging_path=${2:?staging_path required}
  local -r repo_dir="${TEMP_BASE}/${name}"
  local -r repo_name="kubevirt/${name}"

  [[ -d "${staging_path}" ]] || {
    echo "Staging path ${staging_path} does not exist" >&2
    return 1
  }

  git clone "https://${GIT_USER_NAME:-kubevirt-bot}@${GITHUB_FQDN}/${repo_name}.git" "${repo_dir}" || {
    echo "Failed to clone ${repo_name}" >&2
    return 1
  }

  (
    cd "${repo_dir}" || return 1
    git checkout -B "${TARGET_BRANCH}-local"
    git rm -rf --ignore-unmatch .
    git clean -fxd
  )

  cp -rf "${staging_path}/." "${repo_dir}/" || {
    echo "Failed to copy staging files from ${staging_path}" >&2
    return 1
  }
  [[ -f LICENSE ]] && cp -f LICENSE "${repo_dir}/"
  [[ -f .gitignore ]] && cp -f .gitignore "${repo_dir}/"
}

function commit_repo() {
  local -r name=${1:?name required}
  local -r repo_dir="${TEMP_BASE}/${name}"

  (
    cd "${repo_dir}" || return 1
    git config user.email "${GIT_AUTHOR_EMAIL:-kubevirtbot@redhat.com}"
    git config user.name "${GIT_AUTHOR_NAME:-kubevirt-bot}"

    git add -A
    if git diff --cached --quiet; then
      echo "${name} hasn't changed."
      return 0
    fi

    git commit --message "${name} update by KubeVirt Prow build ${BUILD_ID}"
  )
}

function get_pseudo_tag() {
  local -r name=${1:?name required}
  local -r repo_dir="${TEMP_BASE}/${name}"

  (
    cd "${repo_dir}" || return 1
    echo "v0.0.0-$(TZ=UTC0 git show --quiet --date='format-local:%Y%m%d%H%M%S' --format="%cd")-$(git rev-parse --short=12 HEAD)"
  )
}

function push_repo() {
  local -r name=${1:?name required}
  local -r repo_dir="${TEMP_BASE}/${name}"

  (
    cd "${repo_dir}" || return 1

    if [[ -n "${TARGET_TAG}" ]]; then
      if git tag "${TARGET_TAG}" && git push origin "${TARGET_TAG}"; then
        echo "${name} updated for tag ${TARGET_TAG}."
      else
        echo "Failed to push tag ${TARGET_TAG}, may already exist" >&2
        return 1
      fi
    else
      if [[ "${TARGET_BRANCH}" == "main" ]]; then
        git push origin "${TARGET_BRANCH}-local:${TARGET_BRANCH}"
        echo "${name} updated for ${TARGET_BRANCH}."
      fi
    fi
  )
}

function remove_test_files() {
  local -r name=${1:?name required}
  local -r repo_dir="${TEMP_BASE}/${name}"

  find "${repo_dir}" -name "*_test.go" -delete
}

function go_mod_remove_staging() {
  local -r name=${1:?name required}
  local -r repo_dir="${TEMP_BASE}/${name}"
  local -r package_name=${2:?package_name required}

  (
    cd "${repo_dir}" || return 1
    go mod edit -dropreplace "${package_name}"
  )
}

function go_mod_populate_pseudoversion() {
  local -r name=${1:?name required}
  local -r repo_dir="${TEMP_BASE}/${name}"
  local -r package_name=${2:?package_name required}
  local -r package_version=${3:?package_version required}

  (
    cd "${repo_dir}" || return 1
    go mod edit -require "${package_name}@${package_version}"
  )
}

# prepare kubevirt.io/virt-template-api
prepare_repo virt-template-api api
commit_repo virt-template-api

# prepare kubevirt.io/virt-template-client-go
prepare_repo virt-template-client-go staging/src/kubevirt.io/virt-template-client-go
go_mod_remove_staging virt-template-client-go kubevirt.io/virt-template-api
pseudo_tag=$(get_pseudo_tag virt-template-api) || {
  echo "Failed to get pseudo tag for virt-template-api" >&2
  exit 1
}
go_mod_populate_pseudoversion virt-template-client-go kubevirt.io/virt-template-api "${pseudo_tag}"
commit_repo virt-template-client-go

# prepare kubevirt.io/virt-template-engine
prepare_repo virt-template-engine staging/src/kubevirt.io/virt-template-engine
remove_test_files virt-template-engine
go_mod_remove_staging virt-template-engine kubevirt.io/virt-template-api
go_mod_populate_pseudoversion virt-template-engine kubevirt.io/virt-template-api "${pseudo_tag}"
commit_repo virt-template-engine

# push the prepared repos
push_repo virt-template-api
push_repo virt-template-client-go
push_repo virt-template-engine
