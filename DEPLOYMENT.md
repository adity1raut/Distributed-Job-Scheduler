# Deployment Guide

How to actually ship this: the container images, the local/staging stack,
what has to change before any of it is public, and the one piece of this
system's architecture that doesn't fit a "just add replicas" deploy model
— worker-per-organization.

See **[Readme.md](Readme.md)** for local development without Docker, and
**[server/docs/architecture.md](server/docs/architecture.md)** for how the
pieces fit together conceptually. This doc is the operational version of
the same picture.

## Contents

- [What got added](#what-got-added)
- [Quick start (docker compose)](#quick-start-docker-compose)
- [Environment variables — what MUST change](#environment-variables--what-must-change)
- [The worker-per-org reality](#the-worker-per-org-reality)
- [CI](#ci)
- [Deploying beyond a single host](#deploying-beyond-a-single-host)
- [Pre-launch checklist](#pre-launch-checklist)
- [Suggested follow-up changes](#suggested-follow-up-changes)

## What got added

Nothing here existed before — this is new scaffolding, not a fix to
something broken:

| File | Purpose |
|---|---|
| `server/Dockerfile` | Multi-stage build producing both `api` and `worker` binaries; pick which one lands in the final image with `--target api` / `--target worker` |
| `server/.dockerignore` | Keeps `.env`, `.git`, build artifacts out of the build context |
| `ui-interface/Dockerfile` | Builds the Vite app, serves it via nginx |
| `ui-interface/nginx.conf` | SPA fallback (`try_files ... /index.html`) so a hard refresh on `/projects/{id}` doesn't 404, plus asset caching and basic security headers |
| `ui-interface/docker-entrypoint.d/40-render-config.sh` + `config.template.js` | Runtime-injects the API URL into the built frontend — see below, this is the one real code-adjacent change |
| `docker-compose.yml` | Full local/staging stack: postgres, redis, a one-shot `migrate` service, api, worker (opt-in), frontend |
| `.env.deploy.example` | The env vars `docker-compose.yml` reads, with the one required one (`JWT_SECRET`) called out |
| `.github/workflows/ci.yml` | Backend build+vet+test (with real Postgres/Redis services), frontend lint+build, and a job that proves all three Docker images actually build |

All three images were built and smoke-tested against this repo's real
Postgres/Redis while writing this — `/healthz` returns `ok`, the Docker
`HEALTHCHECK` reports `healthy`, the frontend's SPA fallback returns `200`
for a deep link, and its runtime-injected `config.js` reflects whatever
`VITE_API_URL` the container was started with.

### The one code-adjacent change: runtime-configurable API URL

Vite bakes `import.meta.env.VITE_API_URL` into the JS bundle at **build**
time. Taken literally, that means a Docker image built for staging can't
be reused for production — you'd rebuild the same code just to point it
at a different API URL, which defeats the point of building an image once.

`src/api/client.js` now checks a runtime value first:

```js
const BASE_URL = window.__APP_CONFIG__?.API_URL || import.meta.env.VITE_API_URL || 'http://localhost:8080'
```

`window.__APP_CONFIG__` comes from `/config.js`, a plain script tag loaded
before the app. In the Docker image, `docker-entrypoint.d/40-render-config.sh`
(nginx's official image runs every script in that directory before
starting) regenerates that file from the container's `VITE_API_URL` env
var at **startup**. Build once, deploy the same image to as many
environments as you want by changing one env var per container. The
build-time Vite variable still works unchanged for platforms that inject
env vars at build time instead of running this Docker image (Vercel,
Netlify, a plain CI build).

Nothing else about routing or app behavior changed — `App.jsx` is untouched.

## Quick start (docker compose)

```bash
cp .env.deploy.example .env
# edit .env — at minimum, set JWT_SECRET (openssl rand -base64 48)

docker compose up -d postgres redis migrate api frontend
```

Open `http://localhost:5173`, register an org, grab its `org_id` from
DevTools → Local Storage (same as the non-Docker flow in
[Readme.md](Readme.md#8-register-an-account-and-grab-your-org-id)), then:

```bash
WORKER_ORG_ID=<org-id> docker compose --profile worker up -d worker
```

`worker` isn't part of the default `docker compose up` — see
[below](#the-worker-per-org-reality) for why.

## Environment variables — what MUST change

Everything in `internal/config.go` has a working default, listed in full
in the `Project-QA-Prep.pdf` cheat sheet and `server/.env.example`. For
an actual deployment, these are the ones the defaults are wrong for:

| Variable | Dev default | Change to |
|---|---|---|
| `JWT_SECRET` | `dev-secret-change-me` | A real random secret. `docker-compose.yml` refuses to start `api` without one set — it will not silently run with the dev default. |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Your real frontend origin(s), comma-separated. Anything not listed here gets its cross-origin requests blocked by the browser, by design. |
| `DATABASE_URL` | local Postgres, `sslmode=disable` | Your managed Postgres, almost certainly with `sslmode=require` or stricter |
| `REDIS_ADDR` | `localhost:6379` | Your managed Redis. Remember rate limiting fails **open** if this is unreachable (by design — see `server/docs/design-decisions.md`), so a misconfigured address degrades to "no rate limiting," not a crash. |
| `VITE_API_URL` | `http://localhost:8080` | Your real public API URL — set as a container env var, not rebuilt in |

## The worker-per-org reality

This is architectural, not a deployment-script gap: `cmd/worker` refuses
to start without `WORKER_ORG_ID`, and it only ever claims that one org's
jobs (see `server/docs/design-decisions.md#workers-belong-to-exactly-one-organization`
for why — genuine multi-tenant isolation, not an oversight). That means
"deploy the worker" isn't a single step the way "deploy the api" is —
**every organization needs its own running worker process(es)**.

For a handful of orgs, that's manageable by hand: one `docker compose
--profile worker up -d worker` (or one container/pod) per org, each with
its own `WORKER_ORG_ID`. Options as the number of orgs grows:

- **Static, per-org containers/pods.** Simplest, matches today's code
  exactly. Fine until you have dozens of orgs and don't want to hand-manage
  that many container definitions.
- **A small controller process.** Something that lists orgs (a new,
  currently-nonexistent `GET /api/orgs` or a direct DB query) and
  reconciles one running worker container/pod per org — spin one up on
  org creation, tear it down on org deletion. This is real, currently
  unwritten code, not a config change; flagging it here as the natural
  next step if org count grows past "manageable by hand," not implying
  it exists.
- **Kubernetes-native:** a Deployment or Job template parameterized by
  `WORKER_ORG_ID`, created by a small operator/controller reacting to org
  create/delete. Same idea as above, expressed as k8s objects instead of
  raw `docker run`.

None of this is implemented — the docker-compose `worker` service is the
"by hand" option, matching the code as it exists today.

## CI

`.github/workflows/ci.yml` runs on every push/PR:

1. **backend** — `go build`, `go vet`, applies migrations, runs
   `go test ./... -race` against real Postgres + Redis service
   containers (mirrors `server.md#testing` — the integration tests
   self-skip without `TEST_DATABASE_URL`/`TEST_REDIS_ADDR`, both set here).
2. **frontend** — `npm ci`, `npm run lint`, `npm run build`.
3. **docker-build** — builds all three images (`api`, `worker`,
   `frontend`) to prove the Dockerfiles stay buildable. No registry
   push — that needs credentials this repo doesn't have configured yet
   (see [Suggested follow-up changes](#suggested-follow-up-changes)).

## Deploying beyond a single host

`docker-compose.yml` is meant for one box (a VM, a single droplet) or
local staging. Nothing about the images is platform-specific, though —
they run unchanged on:

- **Managed container platforms** (Fly.io, Render, Railway, AWS App
  Runner/ECS): point each at the matching `Dockerfile`/`target`, set the
  env vars above, and use a managed Postgres + managed Redis instead of
  the compose services.
- **Kubernetes**: same three images, one Deployment for `api` (stateless,
  scales freely behind a Service), one Deployment/Job per org for
  `worker` (see above), a Deployment + Service for `frontend` (or serve
  the built `dist/` from a CDN instead of nginx entirely — it's static
  files behind a runtime `config.js`).

Either way, put a real reverse proxy / load balancer in front that
terminates TLS — neither the Go API nor the nginx frontend config here
does HTTPS itself; that's deliberately left to whatever's in front
(a platform's managed LB, or Caddy/Traefik if self-hosting), since the
right answer depends entirely on where this actually runs.

## Pre-launch checklist

- [ ] `JWT_SECRET` set to a real random value, not the dev default
- [ ] `CORS_ALLOWED_ORIGINS` set to the real frontend origin(s)
- [ ] Postgres and Redis are managed/backed-up instances, not the
      throwaway compose containers
- [ ] TLS terminated somewhere in front of both `api` and `frontend`
- [ ] At least one `worker` running per organization that actually needs
      jobs processed
- [ ] `docker compose logs -f api worker` (or your platform's log
      viewer) reads as structured JSON — confirm your log aggregator
      parses `slog`'s JSON output rather than treating it as plain text
- [ ] A real Postgres backup/restore plan — cascading deletes mean a
      bad `DELETE` is not recoverable from within the app itself (see
      `server/docs/design-decisions.md#deletes-cascade-for-now`)

## Suggested follow-up changes

Beyond what's needed to deploy at all, roughly in priority order:

1. **Push images to a registry from CI on tag/main.** The `docker-build`
   CI job proves the images build; it doesn't publish them anywhere yet.
   Add a `docker/login-action` + `docker/build-push-action` step once
   you've picked a registry (GHCR is the zero-extra-setup option for a
   GitHub-hosted repo).
2. **The worker-per-org controller** described above, once org count
   makes hand-managing containers impractical.
3. **Structured metrics** (Prometheus-style `/metrics`) — right now
   observability is JSON logs only. Job throughput, claim latency, and
   queue depth would all be more useful as real time-series than grepped
   from logs.
4. **Externalize the JWT secret properly** — a `.env` file is fine for a
   single host; a real deployment should pull `JWT_SECRET` from a secrets
   manager (AWS Secrets Manager, Vault, or the platform's native secret
   store) instead of an env file sitting on disk.
5. **Automate the retry-policy-creation gap** noted in
   `server/docs/api.md` — not a deployment concern, but worth doing
   before this goes in front of real users who'd want more than one
   backoff strategy per org.
6. **Distroless final images** for `api`/`worker` instead of Alpine, if
   attack surface matters more than the convenience of a shell + `wget`
   for debugging/healthchecks in this environment. Traded off deliberately
   here for a working `HEALTHCHECK` and an easier `docker exec` during
   incident response — revisit if that trade stops making sense for you.
