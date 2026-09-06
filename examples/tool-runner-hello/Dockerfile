# Example sandboxed tool image: hello.
#
# Build (from the core/sdk directory):
#   docker build -t ghcr.io/zeroroot-ai/gibson-tool-runner:hello-dev \
#       -f examples/tool-runner-hello/Dockerfile .
#
# This Dockerfile is the reference for any SDK consumer building their own
# sandboxed tool image: import sdk/toolrunner, write a main.go that calls
# toolrunner.Run(myTool{}), and copy this Dockerfile pointing at your own
# package.
#
# syntax=docker/dockerfile:1.7

# Base images route through the org mirror and are digest-pinned for reproducible
# builds (RESTRUCTURE-QUALITY-BARS §1). Toolchain matches go.mod (Go 1.26.4).
# Digests: golang:1.26.4-alpine + distroless-static-debian12:nonroot from
# ghcr.io/zeroroot-ai/mirror. Re-resolve with `docker buildx imagetools inspect <ref>`.
FROM ghcr.io/zeroroot-ai/mirror/golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/tool-runner ./examples/tool-runner-hello

FROM ghcr.io/zeroroot-ai/mirror/distroless-static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/tool-runner /tool-runner
USER nonroot:nonroot
ENTRYPOINT ["/tool-runner"]
