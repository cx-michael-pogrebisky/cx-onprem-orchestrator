# Slim image: just the cx-onprem-orchestrator binary on distroless.
# Use this when your CI provides the engine tools (cx/kics/2ms/ScaResolver/Java)
# itself, or for the orchestrator's own non-scanning subcommands.
#
#   docker build -t cx-onprem-orchestrator:slim --build-arg VERSION=$(git describe --tags) .
#
# syntax=docker/dockerfile:1
# Alpine build base: our binary is CGO_ENABLED=0, so the static output is identical
# to a glibc build, but the build stage carries far fewer OS-package CVEs (and is
# discarded anyway — the runtime is distroless).
FROM golang:1.26.4-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.0.0-dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/cx-michael-pogrebisky/cx-onprem-orchestrator/internal/cli.Version=${VERSION}" \
      -o /out/cx-onprem-orchestrator ./cmd/cx-onprem-orchestrator

# distroless/static:nonroot ships CA certs (needed for TLS to Checkmarx One) and
# runs as an unprivileged user.
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/cx-onprem-orchestrator /usr/local/bin/cx-onprem-orchestrator
# Explicit non-root user (distroless:nonroot already defaults to uid 65532; this
# satisfies the KICS "Missing User Instruction" check and documents intent).
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/cx-onprem-orchestrator"]
