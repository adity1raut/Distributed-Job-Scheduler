# Job Scheduler Dashboard

React dashboard for the [Distributed Job Scheduler](../Readme.md). Manage
projects and queues, submit and inspect jobs, watch execution logs, and
monitor the worker fleet, all live against the Go API.

Every interactive control (toasts, confirm dialogs, dropdowns) is
hand-built for this app. There's no UI library dependency beyond React
itself, `react-router-dom` for routing, and `lucide-react` for icons.

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
| **Overview** | Org-wide stat cards, an hourly throughput chart, a status-filterable Recent Jobs feed, every project grouped with its queues, and a Worker Pool table. The landing page you don't need to drill into Projects or Workers to read. |

Live views poll the API every few seconds (see `src/hooks/usePolling.js`).
No WebSocket dependency; matches the brief's "polling or WebSockets"
option.

## Setup

```bash
npm install
cp .env.example .env   # set VITE_API_URL if the API isn't on localhost:8080
npm run dev
```

Requires the backend running (see [`../Readme.md`](../Readme.md)). The
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
├── api/            # One file per resource: thin fetch wrappers over the REST API
│                     (jobs, queues, dlq, scheduledJobs, workers, dashboard, projects, auth)
├── components/     # Shared UI
│   ├── Layout.jsx        # Sidebar nav + page frame
│   ├── ToastHost.jsx     # Custom toast notifications (backed by lib/toast.js)
│   ├── ConfirmHost.jsx   # Custom confirm dialog, replaces window.confirm (lib/confirm.js)
│   ├── Select.jsx        # Custom themed dropdown, replaces the native <select>
│   ├── ThroughputChart.jsx  # Stacked-bar completed/failed chart on Overview
│   ├── AuthTabs.jsx, ProtectedRoute.jsx, ErrorBanner.jsx, EmptyState.jsx,
│   │   Skeleton.jsx, StatusBadge.jsx, Metric.jsx, Breadcrumbs.jsx,
│   │   CopyableId.jsx, CopyBlock.jsx, Timestamp.jsx, Wordmark.jsx
│   └── queue/            # Panels composed into the queue detail page
│       # JobSubmitForm + JobsTable (Jobs tab), ScheduledJobsPanel,
│       # DlqPanel, QueueConfigPanel
├── context/
│   └── AuthContext.jsx   # Token/user state, backed by localStorage
├── hooks/
│   └── usePolling.js     # Fetch-on-mount + fetch-on-interval, used by every live view
├── lib/
│   ├── toast.js          # Toast queue/dispatch logic behind ToastHost
│   ├── confirm.js        # Promise-based confirm() replacement behind ConfirmHost
│   └── format.js         # Relative-time and short-ID formatting
├── pages/          # One per route: Dashboard, Projects, ProjectDetail,
│                     QueueDetail, JobDetail, Workers, Login, Register
└── App.jsx         # Routes, plus the app-wide <ToastHost/> and <ConfirmHost/>
```

## Environment variables

| Variable | Default | Meaning |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Base URL the frontend calls for every API request |

## Notes

- **Auth.** The JWT and user object live in `localStorage`. A 401 from
  any request clears them (see `src/api/client.js`), so an expired token
  bounces you back to `/login` on the next fetch rather than silently
  failing.
- **Pagination.** The job explorer follows the API's keyset cursor.
  "Next" pushes the returned `next_cursor`; "Previous" pops a
  client-side stack. Only the first page auto-refreshes; a paged-forward
  view holds still so rows don't shift under you mid-read.
- **Toasts.** Every mutation (create/delete/pause/resume/submit/retry/replay)
  reports success or failure through the custom toast system
  (`lib/toast.js` + `ToastHost.jsx`), styled from the same CSS custom
  properties as the rest of the app (`src/index.css`) so it matches both
  themes.
- **Confirm dialogs.** Destructive actions (deleting a project or queue)
  go through `lib/confirm.js` + `ConfirmHost.jsx` instead of the
  browser's native `window.confirm()`, so they match the app's theme and
  can be dismissed with Escape or a click outside.
