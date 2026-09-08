# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# Build stage
#
# Both binaries are compiled in one stage so the module download and build
# cache are shared. The final image then selects which one to run via its
# command, meaning api and worker ship as a single artifact that is
# guaranteed to be built from the same commit — a mixed-version fleet during
# a rolling deploy is a real source of subtle bugs when the two are built
# separately.
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS build

# git is needed by the Go toolchain for module resolution; ca-certificates
# for fetching modules over HTTPS.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy manifests first so `go mod download` is cached independently of source
# changes — editing a handler must not re-download the module graph.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# CGO_ENABLED=0 produces a statically linked binary with no libc dependency,
# so the runtime image needs nothing but the binary itself.
# -trimpath drops local filesystem paths, keeping builds reproducible.
# -s -w strips the symbol table and DWARF data (~30% smaller).
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.buildVersion=${VERSION}" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.buildVersion=${VERSION}" -o /out/worker ./cmd/worker

# ---------------------------------------------------------------------------
# Runtime stage
#
# Alpine rather than distroless: the compose healthcheck and the debugging
# that inevitably happens during an incident both need a shell and wget. The
# attack surface is kept down instead by running as a non-root user with no
# shell login, a read-only-friendly layout, and Trivy scanning in CI.
# ---------------------------------------------------------------------------
FROM alpine:3.24 AS runtime

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 10001 -S app && \
    adduser -u 10001 -S -G app -H -s /sbin/nologin app

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker

# Migrations travel with the image so a deploy can never run application code
# against a schema the image has not seen.
COPY --chown=root:root migrations /migrations

USER 10001:10001
WORKDIR /

ARG VERSION=dev
ENV APP_VERSION=${VERSION} \
    API_PORT=8080 \
    APP_ENV=production

EXPOSE 8080

# Probes the readiness endpoint, so an api container is only "healthy" once
# Postgres is actually reachable — not merely once the process has started.
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null "http://127.0.0.1:${API_PORT}/readyz" || exit 1

CMD ["api"]
