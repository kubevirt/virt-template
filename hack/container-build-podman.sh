#!/bin/bash

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
