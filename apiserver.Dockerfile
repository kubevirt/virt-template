# Build the apiserver binary
ARG BUILD_ARCH=amd64
FROM --platform=linux/${BUILD_ARCH} docker.io/library/golang:1.24 AS builder

ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY go.work go.work
COPY go.work.sum go.work.sum

# Copy the go source
COPY cmd/apiserver/main.go cmd/apiserver/main.go
COPY api/ api/
COPY internal/ internal/
COPY staging/ staging/
COPY vendor/ vendor/

# Copy version information
COPY _out/version _out/version
COPY hack/ldflags.sh hack/ldflags.sh
COPY hack/version.sh hack/version.sh

# Build
RUN --mount=type=cache,id="virt-template",target="/root/.cache/go-build" \
    mkdir -p bin && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
        -ldflags "$(hack/ldflags.sh)" \
        -o bin/apiserver cmd/apiserver/main.go

# Use distroless as minimal base image to package the apiserver binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM --platform=linux/${TARGETARCH} gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/apiserver .
USER 65532:65532

ENTRYPOINT ["/apiserver"]
