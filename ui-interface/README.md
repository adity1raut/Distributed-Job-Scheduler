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

## Testing

```bash
npm run lint
```

There's no automated test suite on the frontend (no Jest/Vitest, no
component tests). Lint is the only automated check. For actually
verifying the UI's behavior, walk through the checklist below against a
running instance.

## Verifying the Frontend UI

`go test` and the API only prove the backend is correct. Neither touches
the dashboard. With the API, a worker, and `npm run dev` running (see
[Setup](#setup)), open the app and walk through this list. Each row is
something to click, not just read.

| Area | What to check |
|---|---|
| **Dropdowns** | Open the job-type selector (Jobs tab) or the status filter. It opens a themed popover menu that matches light/dark mode, not the browser's native OS-style option list. |
| **Toasts** | Do anything that mutates state (create a project, submit a job, pause a queue). A toast slides in from the top-right with a colored left rule and a shrinking progress bar; hovering it pauses the auto-dismiss timer. |
| **Confirm dialog** | Projects → **Delete** on a project card. A centered modal with a warning icon and a solid-red **Delete** button appears, not the browser's native `confirm()` popup. Escape or clicking outside cancels it. |
| **Sidebar nav** | Overview / Projects / Workers each have an icon, and the active page is a filled amber pill, not just a text color change. |
| **Section tabs** | On a queue's detail page, Jobs / Scheduled / Dead letters / Configuration render as a segmented pill control: the active tab sits raised on its own background inside a bordered track. |
| **Auth tabs** | On `/login` or `/register`, a "Log in / Register" tab pair sits above the form and switches pages when clicked. |
| **Tables** | Job/worker/queue listings have a filled header bar and roomy rows, not a cramped, thin-text grid. |
| **Number fields** | Priority, concurrency, delay (ms), and batch-count inputs show no up/down spinner arrows, just plain numeric fields. |
| **Scrollbars** | Open a dropdown with more options than fit (e.g. the status filter). The scrollbar is a thin, theme-colored bar, not the platform default. |
| **Headings** | Page titles ("Overview", "Projects", a queue's name, "Job detail") are visibly larger and bolder than the body text under them. |

For the underlying job-scheduling behavior itself (delays, concurrency
limits, retries, dead-lettering, cron dispatch) rather than the UI chrome,
follow the full click-by-click script in
[`../WORKING.md`](../WORKING.md#part-2--verifying-it-all-yourself-locally-in-the-ui).

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
- **Rate limiting and continuous polling — verified working.** The
  dashboard is polling-heavy: a page like Overview alone runs five
  independent `usePolling` calls (`getOverview` every 5s, plus
  workers/throughput/project lists on their own intervals), each a
  separate request against the API's per-org rate limiter
  (`RATE_LIMIT_PER_MIN`, default 120/min, see
  [`server/README.md`](../server/README.md#testing)). The limiter used to
  reset its 60s window on *every* request instead of only the first one
  in a window, so continuous polling kept pushing the expiry forward and
  the count never reset — real dashboard usage would get wrongly stuck
  on `429 rate limit exceeded` after a couple of minutes. That's fixed
  server-side (`internal/middleware/ratelimit.go`); with the fix, leaving
  the dashboard open and polling continuously across multiple tabs/pages
  no longer trips a false rate limit. The frontend itself needed no
  change — a `429` just surfaces through the normal `error` state /
  toast path in `src/api/client.js`, same as any other API error.
