# Deployment Guide

Everything in this project runs in Docker containers, and GitHub Actions
builds, tests and ships them. There is no Kubernetes and nothing else to
learn — if you know `docker compose up`, you can run this.

## Contents

- [What runs where](#what-runs-where)
- [Running it locally](#running-it-locally)
- [How the pieces fit together](#how-the-pieces-fit-together)
- [Configuration](#configuration)
- [Deploying to a real server](#deploying-to-a-real-server)
- [The CI/CD pipeline](#the-cicd-pipeline)
- [Day-two operations](#day-two-operations)
- [Troubleshooting](#troubleshooting)

---

## What runs where

Six containers. Two of them do their job and exit; four keep running.

| Container | Image | What it does | Lifetime |
|---|---|---|---|
| `postgres` | `postgres:17-alpine` | Stores everything: orgs, queues, jobs, results | long-running |
| `redis` | `redis:7-alpine` | Rate-limit counters only | long-running |
| `migrate` | `migrate/migrate` | Applies SQL migrations, then exits | runs once |
| `api` | built from `server/` | REST API + the cron scheduler | long-running |
| `bootstrap` | `curlimages/curl` | Creates a first org, then exits | runs once |
| `worker` | built from `server/` | Claims jobs and runs them | long-running |
| `web` | built from `ui-interface/` | nginx: serves the dashboard, proxies `/api` | long-running |

`api` and `worker` are the **same image** with a different command. They are
built together from one Dockerfile, so they can never be running mismatched
versions of the code.

---

## Running it locally

```bash
make up
```

That is genuinely all of it. The target copies `.env.example` to `.env` if you
have no `.env` yet, builds both images, and starts everything in dependency
order.

Then open **http://localhost:3000** and sign in with the credentials from your
`.env` (`admin@example.com` / `password123` by default).

To prove it actually works end to end — not just that containers started:

```bash
make smoke
```

That submits a real job through the dashboard's URL and waits for a worker to
finish it. If that passes, every part of the system is working.

### Everyday commands

```bash
make up                    # start everything
make down                  # stop, keep the database
make clean                 # stop and DELETE the database
make logs                  # tail all logs
make logs SVC=worker       # tail just the workers
make status                # what is running
make psql                  # open a SQL shell
make scale-workers N=5     # run 5 workers instead of 2
make help                  # every available command
```

---

## How the pieces fit together

### Startup order

Containers do not start all at once. Each waits for what it needs:

```
postgres  ──healthy──┐
                     ├──> migrate ──exits 0──> api ──healthy──> web
redis     ──healthy──┘                          │
                                                └──> bootstrap ──exits 0──> worker
```

This ordering is enforced with `depends_on` + `condition`, not with
`sleep`. Two parts are worth understanding:

**Migrations run in their own container.** They are not part of the API's
startup. If migrations were inside the API, every replica would race to
migrate the same database at boot, and a broken migration would show up as a
crash-looping API instead of a clearly failed deploy step. As a separate
one-shot container, a failed migration stops the deploy with an obvious error
and the API never starts against a schema it does not expect.

**`bootstrap` exists to solve a chicken-and-egg problem.** A worker must know
which organization it serves — it only claims that org's jobs. But an org only
exists after somebody registers. Without `bootstrap`, `docker compose up` would
leave you with a worker that refuses to start until you manually registered
through the UI and copy-pasted a UUID into `.env`. So `bootstrap` registers an
org over the API, writes its ID to a shared volume, and the worker reads it
from there. It is idempotent: run it again and it logs in instead of
registering.

In production this service is disabled — see [Deploying to a real
server](#deploying-to-a-real-server).

### Why the dashboard and API share one origin

The `web` container runs nginx, which does two jobs:

1. Serves the built React app.
2. Reverse-proxies `/api/*` to the `api` container.

So the browser only ever talks to **one** address. It never makes a
cross-origin request, which means **CORS never comes into it** and there is no
per-environment allowlist to maintain. It also means the API does not need to
be exposed to the internet at all — only nginx does.

### One image, every environment

Vite bakes `VITE_*` variables in at **build** time. If the API URL were a build
variable, you would need a separate image per environment, and the image you
tested in staging would not be the one you shipped.

Instead, the container writes `/config.js` at **startup** from the `API_URL`
environment variable, and the app reads it from `window.__APP_CONFIG__`. One
image is built once and promoted unchanged. `API_URL` defaults to empty, which
means "same origin" — the nginx proxy above.

### Health endpoints

| Path | Answers | Checks |
|---|---|---|
| `/livez` | Is the process alive? | nothing external |
| `/readyz` | Can it serve a request? | Postgres, Redis |
| `/healthz` | alias of `/livez` | — |

`/livez` deliberately checks **nothing external**. A restart cannot fix a down
database, so if liveness failed when Postgres was down, one database outage
would turn into every container restart-looping — turning a recoverable
problem into a much worse one.

`/readyz` does check Postgres, which is what Docker's healthcheck uses. A
container that cannot reach the database is marked unhealthy and a deploy will
not proceed past it.

Redis is checked but marked non-critical, because the rate limiter is written
to fail open: if Redis is down, requests are allowed through rather than
rejected. Losing Redis costs you rate limiting, not uptime.

---

## Configuration

All configuration is environment variables. `.env.example` is the template;
copy it to `.env` and edit. `.env` is gitignored and must never be committed.

### Must change before production

| Variable | Why | Generate with |
|---|---|---|
| `JWT_SECRET` | Signs every login token. The default is public. | `openssl rand -base64 48` |
| `POSTGRES_PASSWORD` | Database access | `openssl rand -base64 24` |
| `REDIS_PASSWORD` | Redis has no user model; this is the whole boundary | `openssl rand -base64 24` |
| `WORKER_ORG_ID` | The org whose jobs your workers run | from the register/login response |

```bash
make secrets     # prints all three, freshly generated
```

Changing `JWT_SECRET` invalidates every issued token, so everyone is logged
out. That is exactly what you want if it ever leaks.

### `TRUSTED_PROXIES` — worth understanding

The API rate-limits unauthenticated requests (login, register) per IP address.
Behind nginx, every request arrives from nginx's IP, so **all** login attempts
in the world would share one bucket — one noisy client could lock everyone
out, and per-IP brute-force protection would be gone.

The fix is `X-Forwarded-For`, but that header is trivially forged by clients.
So the API honours it **only** when the request came from an address listed in
`TRUSTED_PROXIES`. With the list empty, the header is ignored entirely, which
is the correct default for an API exposed directly.

Set it to the network your proxy is on. The compose default covers the Docker
bridge ranges.

### Ports

The dev setup publishes Postgres on **5433** and Redis on **6380**, not their
usual ports, so the stack coexists with a Postgres or Redis already installed
on your machine. They are bound to `127.0.0.1` — not reachable from your
network. In production neither is published at all.

---

## Deploying to a real server

You need a Linux box with Docker installed. That is the only requirement.

### 1. Prepare the server

```bash
sudo mkdir -p /opt/job-scheduler && cd /opt/job-scheduler
```

Copy `docker-compose.yml`, `docker-compose.prod.yml`, `server/migrations/` and
`deploy/scripts/` there. (The CD pipeline does this for you — see below.)

### 2. Create `.env` on the server

Real secrets live on the server and are never in git:

```bash
cat > .env <<EOF
IMAGE_REGISTRY=ghcr.io
IMAGE_NAMESPACE=adity1raut/distributed-job-scheduler
IMAGE_TAG=v1.0.0

POSTGRES_PASSWORD=$(openssl rand -base64 24)
REDIS_PASSWORD=$(openssl rand -base64 24)
JWT_SECRET=$(openssl rand -base64 48)

WORKER_ORG_ID=          # fill in after step 4
TRUSTED_PROXIES=172.16.0.0/12
WEB_PORT=3000
EOF
chmod 600 .env
```

### 3. Start it

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

**Always pass both `-f` flags.** Compose applies `docker-compose.override.yml`
automatically when you do not, and that file publishes your database port.
`make deploy` gets this right for you.

### 4. Get your org ID

`bootstrap` is disabled in production — it would create an org with a known
default password. Register properly through the dashboard instead, then copy
the `org_id` from the response (browser dev tools → Network tab, or Local
Storage under `user`), put it in `.env` as `WORKER_ORG_ID`, and restart the
workers:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d worker
```

### 5. Put HTTPS in front

The prod overlay binds the dashboard to `127.0.0.1:3000` — deliberately **not**
reachable from the internet, because it speaks plain HTTP. Terminate TLS with a
reverse proxy on the host. Caddy is the least work:

```
jobs.example.com {
    reverse_proxy 127.0.0.1:3000
}
```

Caddy obtains and renews the certificate automatically. nginx or Traefik work
equally well.

### What the prod overlay changes

- Images come from the registry instead of being built on the server
- Database and Redis ports are not published at all
- CPU and memory limits on every container, so one runaway container cannot
  take the host down
- `read_only: true` root filesystems on the API and workers
- `no-new-privileges` — blocks privilege escalation inside a container
- `bootstrap` disabled
- Two API replicas, so a restart never drops to zero

Check what it resolves to without starting anything:

```bash
make deploy-check
```

---

## The CI/CD pipeline

Three workflows in `.github/workflows/`.

### `ci.yml` — runs on every push and pull request

| Job | What it does |
|---|---|
| `backend` | gofmt, `go vet`, golangci-lint, tests with `-race` against a **real** Postgres and Redis |
| `frontend` | `npm ci`, lint, build |
| `images` | Builds both Docker images, scans them with Trivy for CVEs |
| `e2e` | Starts the whole compose stack and runs the smoke test |

The `backend` job runs Postgres and Redis as service containers rather than
mocks, because the behaviour this project depends on — `SELECT ... FOR UPDATE
SKIP LOCKED`, advisory locks, unique constraints — cannot be faked. The
integration tests **skip themselves silently** when `TEST_DATABASE_URL` is
unset, so without real services CI would go green having tested almost nothing.

The `e2e` job is the one that matters most. Unit tests passing while the
deployment is broken is a real and common failure; this catches it by starting
the actual stack and submitting an actual job.

### `cd.yml` — runs on pushes to master and on version tags

1. Builds both images for **amd64 and arm64** and pushes them to GitHub
   Container Registry.
2. Tags each image with the commit SHA, and with the semver version on a tag
   push. The SHA tag is what deployments reference — `latest` moves, so you
   cannot roll back to it or say what is running.
3. Attaches build provenance, so any deployed image traces back to the exact
   workflow run and commit that produced it.
4. If `DEPLOY_HOST` is configured, SSHes to the server, pulls the new images,
   runs migrations, and rolls the stack.
5. Runs the smoke test against the deployed site.
6. **If the smoke test fails, it rolls back to the previous tag automatically.**

A deploy that passes health checks but cannot actually run a job is still a
failed deploy — hence step 5 gating step 6.

If no `DEPLOY_HOST` is set, it publishes images and skips deployment cleanly,
so the workflow is useful before any server exists.

#### To enable automatic deployment

In your repo settings → Environments, create `staging` and/or `production`,
then add:

| Type | Name | Value |
|---|---|---|
| Secret | `DEPLOY_HOST` | server IP or hostname |
| Secret | `DEPLOY_USER` | SSH user |
| Secret | `DEPLOY_SSH_KEY` | private key for that user |
| Secret | `SMOKE_EMAIL` | an existing account for the smoke test |
| Secret | `SMOKE_PASSWORD` | its password |
| Variable | `DEPLOY_URL` | `https://jobs.example.com` |
| Variable | `DEPLOY_PATH` | `/opt/job-scheduler` |

Put required reviewers on the `production` environment and deploys wait for
approval.

### `codeql.yml`

GitHub's static analysis on Go and JavaScript, plus a weekly scheduled run so
newly published rules get applied to existing code.

### `dependabot.yml`

Weekly dependency PRs for Go, npm, Docker base images and GitHub Actions.
Minor and patch updates are grouped into one PR instead of twenty.

### Releasing a version

```bash
git tag v1.0.0
git push origin v1.0.0
```

That triggers a build tagged `v1.0.0`, `1.0`, and the commit SHA, then deploys
to production.

---

## Day-two operations

### Scaling workers

Workers are stateless and claim jobs with `SELECT ... FOR UPDATE SKIP LOCKED`,
so two workers can never take the same job. Add as many as you like:

```bash
make scale-workers N=8
```

Remember each worker holds its own database connection pool — scaling workers
scales database connections too. Watch `max_connections`.

### Rolling out a new version

```bash
git tag v1.1.0 && git push origin v1.1.0     # CD does the rest
```

Or manually on the server:

```bash
cd /opt/job-scheduler
sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=v1.1.0/' .env
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml run --rm migrate
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

Migrations run **before** the new containers start, as their own step.

### Rolling back

```bash
sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=v1.0.0/' .env
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

This rolls back **code**, not schema. Write migrations so the previous version
still works against the new schema — add columns, do not rename or drop them in
the same release. A rename becomes two deploys: add the new column and write to
both, then drop the old one once nothing reads it.

### Backups

The compose stack does not back anything up. Set this up before you have data
worth losing:

```bash
docker compose exec -T postgres pg_dump -U postgres jobscheduler | gzip > backup-$(date +%F).sql.gz
```

Put it in cron, ship it off the machine, and **restore from it once** to
confirm the backup is real. An untested backup is not a backup.

### Graceful shutdown

`stop_grace_period: 45s` on the worker exists because `cmd/worker` waits up to
30 seconds for in-flight jobs to finish on SIGTERM. Docker's 10-second default
would kill jobs mid-execution and force a retry.

---

## Troubleshooting

**`make up` fails: `address already in use`**

Something already holds the port. The dev setup uses 5433 and 6380 to avoid
this, but 3000 or 8080 may be taken:

```bash
ss -ltnp | grep -E ':(3000|8080|5433|6380)'
```

Change `WEB_PORT` or `API_PORT` in `.env`.

---

**`api` container is unhealthy**

```bash
docker compose logs api
docker compose exec api wget -qO- http://127.0.0.1:8080/readyz
```

`/readyz` returns exactly which dependency failed and why.

---

**Jobs sit in `queued` and never run**

Almost always the org ID. A worker claims jobs from **one** organization only.
If `WORKER_ORG_ID` does not match the org you submitted under, the worker sits
idle while the job waits forever.

```bash
docker compose logs worker | head -20     # shows the org_id it registered with
```

Compare it against the `org_id` in your login response. Also check the queue is
not paused — the Workers page in the dashboard shows online workers.

---

**`exec /usr/local/bin/api: operation not permitted`**

Some Docker builds reject `exec` under `no-new-privileges`. Confirm it is the
daemon and not this project:

```bash
docker run --rm --security-opt no-new-privileges:true alpine echo ok
```

If that also fails, your Docker installation is affected. This is why the
hardening lives only in `docker-compose.prod.yml` — plain `make up` does not use
it. Either upgrade Docker or remove the `security_opt` blocks from the prod
overlay.

---

**Everything is broken and I want a clean slate**

```bash
make clean && make up
```

`make clean` **deletes the database volume**. Never run it on a server with
real data.
