# Job Scheduler Dashboard

React dashboard for the [Distributed Job Scheduler](../Readme.md) — manage
projects and queues, submit and inspect jobs, watch execution logs, and
monitor the worker fleet, all live against the Go API.

## Features

| Area | What it does |
|---|---|
| **Auth** | Register (creates an org + owner user) / log in, JWT stored client-side |
| **Projects** | Create, list, delete |
| **Queues** | Create, configure (priority, concurrency limit), pause/resume, live per-status counts |
| **Job explorer** | Submit immediate/delayed/scheduled/batch jobs, filter by status, paginate, click through to detail |
| **Job detail** | Payload, last error, attempt timeline, per-attempt execution history and logs, manual retry |
| **Scheduled jobs** | Create/pause/resume cron definitions |
| **Dead-letter queue** | Inspect permanently-failed jobs, one-click replay |
| **Workers** | Fleet status (online/stale), heartbeat history |
| **Overview** | Org-wide stat cards (queue depth, throughput, failures, dead-lettered, online workers), a status-filterable Recent Jobs feed across every project, every project grouped with its queues, and a Worker Pool table — the landing page you don't need to drill into Projects or Workers to read |

Live views poll the API every few seconds (see `src/hooks/usePolling.js`) —
no WebSocket dependency, matches the brief's "polling or WebSockets" option.

## Setup

```bash
npm install
cp .env.example .env   # set VITE_API_URL if the API isn't on localhost:8080
npm run dev
```

Requires the backend running (see [`../Readme.md`](../Readme.md)) — the
dashboard has nothing to render without it.

## Scripts

| Command | Does |
|---|---|
| `npm run dev` | Start the Vite dev server (HMR) |
| `npm run build` | Production build to `dist/` |
| `npm run preview` | Serve the production build locally |
| `npm run lint` | ESLint |

## Structure

```
src/
├── api/            # One file per resource — thin fetch wrappers over the REST API
├── components/     # Shared UI: Layout (sidebar), Breadcrumbs, StatusBadge,
│                     StatCard, CopyableId, CopyBlock, EmptyState, Skeleton, Timestamp
│   └── queue/        # Panels composed into the queue detail page (Jobs, Scheduled,
│                       Dead letters, Configuration)
├── context/        # AuthContext — token/user state, backed by localStorage
├── hooks/
│   └── usePolling.js # Fetch-on-mount + fetch-on-interval, used by every live view
├── lib/
│   └── format.js     # Relative-time and short-ID formatting
├── pages/          # One per route
└── App.jsx         # Routes + the app-wide <Toaster/>
```

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Base URL the frontend calls for every API request |

## Notes

- **Auth:** the JWT and user object live in `localStorage`; a 401 from any
  request clears them (see `src/api/client.js`), so an expired token bounces
  you back to `/login` on the next fetch rather than silently failing.
- **Pagination:** the job explorer follows the API's keyset cursor — "Next"
  pushes the returned `next_cursor`, "Previous" pops a client-side stack.
  Only the first page auto-refreshes; a paged-forward view holds still so
  rows don't shift under you mid-read.
- **Toasts:** every mutation (create/delete/pause/resume/submit/retry/replay)
  reports success or failure via `react-hot-toast`, styled from the same CSS
  custom properties as the rest of the app (`src/index.css`) so it matches
  both themes.
