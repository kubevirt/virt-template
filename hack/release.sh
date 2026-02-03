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

set -exuo pipefail

GH_CLI_DIR=""
GH_CLI_VERSION="${GH_CLI_VERSION:-2.83.2}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-kubevirt/virt-template}"

: "${GPG_USER_ID:?GPG_USER_ID must be set}"

function cleanup_gh_install() {
    [ -n "${GH_CLI_DIR}" ] && [ -d "${GH_CLI_DIR}" ] && rm -rf "${GH_CLI_DIR:?}/"
}

function ensure_gh_cli_installed() {
    if command -V gh; then
        return
    fi

    trap 'cleanup_gh_install' EXIT SIGINT SIGTERM

    # install gh cli for uploading release artifacts, with prompt disabled to enforce non-interactive mode
    GH_CLI_DIR=$(mktemp -d)
    (
        cd "${GH_CLI_DIR}/"
        curl -sSL "https://github.com/cli/cli/releases/download/v${GH_CLI_VERSION}/gh_${GH_CLI_VERSION}_linux_amd64.tar.gz" -o "gh_${GH_CLI_VERSION}_linux_amd64.tar.gz"
        tar xvf "gh_${GH_CLI_VERSION}_linux_amd64.tar.gz"
    )
    export PATH="${GH_CLI_DIR}/gh_${GH_CLI_VERSION}_linux_amd64/bin:$PATH"
    if ! command -V gh; then
        echo "gh cli not installed successfully"
        exit 1
    fi
    gh config set prompt disabled
}

function build_release_artifacts() {
    export VERSION="${TARGET_TAG}"
    make
    make build-installer
    make build-installer-openshift
    make build-installer-virt-operator
    make build-virttemplatectl
    make container-build
    make container-push
    sha256sum dist/install*.yaml bin/virttemplatectl-"${TARGET_TAG}"-* | sed 's|  .*/|  |' > dist/CHECKSUMS.sha256
}

function update_github_release() {
    # note: for testing purposes we set the target repository, gh cli seems to always automatically choose the
    # upstream repository automatically, even when you are in a fork
    set +e
    if ! gh release view --repo "${GITHUB_REPOSITORY}" "${TARGET_TAG}"; then
        set -e
        git tag -l --format='%(contents)' "${TARGET_TAG}" > /tmp/tag_notes
        gh release create --repo "${GITHUB_REPOSITORY}" "${TARGET_TAG}" --prerelease --title="${TARGET_TAG}" --notes-file /tmp/tag_notes
    else
        set -e
    fi

    gh release upload --repo "${GITHUB_REPOSITORY}" --clobber "${TARGET_TAG}" \
        bin/virttemplatectl-"${TARGET_TAG}"-* \
        dist/install.yaml \
        dist/install-openshift.yaml \
        dist/install-virt-operator.yaml \
        dist/CHECKSUMS.sha256
}

function update_github_source_tarball_signature() {
    local src_tarball_file
    src_tarball_file="${TARGET_TAG}.tar.gz"
    local src_tarball_signature_file
    src_tarball_signature_file="${src_tarball_file}.asc"

    if ! gh release download "${TARGET_TAG}" --repo "${GITHUB_REPOSITORY}" --pattern="${src_tarball_signature_file}" --clobber --output "/tmp/${src_tarball_signature_file}"; then
        upload_github_source_tarball_signature
    else
        gh release download "${TARGET_TAG}" --repo "${GITHUB_REPOSITORY}" --archive=tar.gz --clobber --output "/tmp/${src_tarball_file}"
        if ! gpg --verify "/tmp/${src_tarball_signature_file}" "/tmp/${src_tarball_file}"; then
            upload_github_source_tarball_signature
        fi
    fi
}

function upload_github_source_tarball_signature() {
    local src_tarball_file
    src_tarball_file="${TARGET_TAG}.tar.gz"

    # 1. download source tarball if not already present
    if [ ! -f "/tmp/${src_tarball_file}" ]; then
        gh release download "${TARGET_TAG}" --repo "${GITHUB_REPOSITORY}" --archive=tar.gz --output "/tmp/${src_tarball_file}"
    fi

    # 2. sign with private key (to verify the signature a prerequisite is that the public key is uploaded)
    [ -f "/tmp/${src_tarball_file}.asc" ] && rm "/tmp/${src_tarball_file}.asc"
    gpg --armor --detach-sign --local-user "${GPG_USER_ID}" --output "/tmp/${src_tarball_file}.asc" "/tmp/${src_tarball_file}"

    # 3. upload the detached signature
    gh release upload --repo "${GITHUB_REPOSITORY}" --clobber "${TARGET_TAG}" "/tmp/${src_tarball_file}.asc"
}

function main() {
    TARGET_TAG="$(git tag --points-at HEAD | head -1)"
    if [ -z "${TARGET_TAG}" ]; then
        echo "commit $(git show -s --format=%h) doesn't have a tag, exiting..."
        exit 0
    fi
    export TARGET_TAG

    ensure_gh_cli_installed

    gh auth login --with-token <"${GITHUB_TOKEN:-/etc/github/oauth}"

    build_release_artifacts
    update_github_release
    update_github_source_tarball_signature

    hack/publish-staging.sh
}

main "$@"
