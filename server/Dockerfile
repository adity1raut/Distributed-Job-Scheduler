# syntax=docker/dockerfile:1
#
# Builds both server binaries from one image. Select which one ends up in
# the final image with `--target api` or `--target worker` (docker-compose.yml
# does this via `build.target`).

FROM golang:1.25-alpine AS builder
WORKDIR /src

# Cached separately from the source copy so `go mod download` only reruns
# when go.mod/go.sum actually change, not on every source edit.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# Shared minimal base: CA certs (TLS to a managed Postgres/Redis), tzdata
# (cron schedule evaluation), and a non-root user.
FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app
USER app

FROM base AS api
COPY --from=builder /out/api /usr/local/bin/api
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/api"]

FROM base AS worker
COPY --from=builder /out/worker /usr/local/bin/worker
# No HTTP surface to probe here by design — a worker's liveness is its
# heartbeat row, already visible on the dashboard's Workers page and
# enforced by the scheduler's own STALE_JOB_SEC reaper. A container-level
# healthcheck would just be reimplementing that check with less information.
ENTRYPOINT ["/usr/local/bin/worker"]
