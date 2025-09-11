# Build the manager binary
ARG BUILD_ARCH=amd64
FROM --platform=linux/${BUILD_ARCH} golang:1.24 AS builder

ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY go.work go.work
COPY go.work.sum go.work.sum

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/
COPY staging/ staging/
COPY vendor/ vendor/

# Build
RUN --mount=type=cache,id="virt-template",target="/root/.cache/go-build" \
    mkdir -p bin && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o bin/manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM --platform=linux/${TARGETARCH} gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
