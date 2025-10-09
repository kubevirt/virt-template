#!/bin/bash

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
