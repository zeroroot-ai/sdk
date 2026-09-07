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
# builds (RESTRUCTURE-QUALITY-BARS §1). The builder names the Go that go.mod
# names; the org toolchain guard (reusable-go-ci) fails the PR when they drift,
# and GOTOOLCHAIN=local makes a mismatch fail the build instead of downloading
# a toolchain. Re-resolve digests with `crane digest <ref>`.
FROM ghcr.io/zeroroot-ai/mirror/golang:1.26.8-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS build
ARG GOTOOLCHAIN=local
ENV GOTOOLCHAIN=${GOTOOLCHAIN}
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
