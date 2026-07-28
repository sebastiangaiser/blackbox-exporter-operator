# Build the manager binary.
# --platform=$BUILDPLATFORM keeps the builder native to the runner and lets Go
# cross-compile to TARGETARCH, avoiding slow QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.26.5 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the Go source (relies on .dockerignore to filter)
COPY . .

# Cross-compile; cache mounts persist the module and build caches across builds.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
