# Feature Handoff: Web UI

**ID:** fh-008
**Feature:** Web UI
**Components:** `web/` — React + Vite + shadcn/ui, embedded in the Go binary via `go:embed`

---

## Overview

The Carto Web UI is a single-page application (SPA) built with React, Vite, and shadcn/ui. It provides a visual interface for managing projects, triggering indexing, monitoring progress via SSE, querying the semantic index, and configuring the system. The compiled SPA assets are embedded into the Go binary using `go:embed`, so `carto serve` serves both the API and the UI from a single process with no separate frontend deployment required.

---

## Pages

### Dashboard

The landing page. Displays a data table of all registered projects with their status, last indexed time, and module count. Uses the shadcn/ui `Table` component. Columns are responsive; on small screens, less critical columns are hidden.

### Index

A compact inline form for triggering indexing. The user selects a project, optionally specifies a module or full re-index, and clicks to start. During indexing, live progress is displayed via SSE integration. A stop button allows the user to cancel an in-progress index.

### Query

An inline query interface with filters for project, retrieval tier (`mini`, `standard`, `full`), and result count. Results are displayed below the query form. The interface supports iterative querying without page reloads.

### Project Detail

A two-column layout. The left column shows project metadata (name, path, status, last indexed). The right column contains a sources editor for configuring external signal sources (e.g., GitHub, Jira). Module listing is available below the metadata.

### Settings

A two-column grid layout for system configuration. Displays current LLM provider settings, Memories URL, and other configuration values. Supports editing configuration via `PUT /api/config`.

---

## Architecture

### Embedded SPA

The React app is built with Vite during development and at compile time. The built assets (`dist/`) are embedded into the Go binary using `go:embed` directives in the server package. At runtime, the server serves these static files from the embedded filesystem. Any request that does not match an `/api/` route is served the SPA's `index.html`, enabling client-side routing.

### Component Library

The UI uses [shadcn/ui](https://ui.shadcn.com/) components, including:
- **Table** — Project listing on the Dashboard.
- **Switch** — Toggle controls in Settings.
- **Tooltip** — Contextual help throughout the UI.

### Sidebar

An icon-only collapsible sidebar provides navigation between pages. It collapses to save space and is suitable for narrow viewports.

### Responsive Design

The UI is mobile-friendly. On small screens, table columns are selectively hidden, layouts switch from multi-column to single-column, and the sidebar collapses.

### SSE Integration

The Index page establishes an `EventSource` connection to `POST /api/projects/{name}/index` (via a fetch-based SSE client, since native `EventSource` only supports GET). It renders progress events in real time, showing the current phase, progress percentage, and status messages. On `complete`, `error`, or `stopped` events, the connection closes and the UI updates accordingly.

---

## Configuration

| Setting | Source | Description |
|---------|--------|-------------|
| Port | `carto serve --port` | The port for the combined API + UI server. Default: `8950`. |
| Projects dir | `carto serve --projects-dir` | Base directory for project discovery. |

The UI itself has no separate configuration. It communicates with the API at the same origin.

---

## Edge Cases

- **Memories server unavailable:** The Dashboard and Project Detail pages display error states when API calls fail. The UI shows a user-facing error message (e.g., "Failed to load projects") rather than a blank page. The Settings page may still render partially.
- **Large project lists:** The Dashboard table does not currently paginate. With hundreds of projects, the table renders all rows, which may impact performance. Pagination or virtual scrolling would address this.
- **SSE reconnection:** If the SSE connection drops during indexing (e.g., network interruption), the `EventSource` client attempts to reconnect. However, the server-side index operation continues to completion (or cancellation). Reconnecting yields a new stream that does not replay past events, so the UI may show an incomplete progress view.
- **Stale UI data:** After indexing completes, the Dashboard does not automatically refresh project status. The user must reload the page or navigate away and back to see updated data.
- **Browser compatibility:** The UI targets modern browsers (Chrome, Firefox, Safari, Edge). SSE, `fetch`, and ES modules are required.

---

## Common Questions

**Q1: How do I access the Web UI?**
Run `carto serve` and open `http://localhost:8950` in a browser. The UI is served from the same port as the API.

**Q2: Can I customize the UI?**
The UI source is in the `web/` directory. To modify it, edit the React components, rebuild with `npm run build` (or `pnpm build`), and recompile the Go binary. The embedded assets update when the Go binary is rebuilt.

**Q3: How do I add a new page?**
Add a new React component in `web/src/pages/`, add a route in the router, and add a sidebar entry. Rebuild the SPA and recompile the Go binary.

**Q4: Does the UI work on mobile?**
Yes. The layout is responsive. The sidebar collapses, table columns are hidden on narrow screens, and forms stack vertically.

**Q5: How does data refresh in the UI?**
Pages fetch data from the API on mount. The Index page uses SSE for real-time updates during indexing. Other pages require a manual refresh or navigation event to reload data. There is no WebSocket-based live update mechanism outside of the SSE index flow.

---

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---------|-------------|------------|
| Blank page at `localhost:8950` | Embedded SPA assets missing or corrupted. Build may have failed. | Rebuild the SPA (`npm run build` in `web/`) and recompile the Go binary. Check that `go:embed` directives include the `dist/` output. |
| API errors in browser console | Backend not running, wrong port, or CORS issue during development. | Verify `carto serve` is running. During development, ensure the Vite proxy is configured for `/api/` routes. |
| SSE progress not updating | EventSource connection failed or was blocked by a proxy. | Check the browser's Network tab for the SSE connection. Ensure no proxy is buffering or closing the stream. |
| Settings not saving | `PUT /api/config` returned an error. | Check the browser console for the error response. Verify the configuration values are valid. Check the server logs. |
| Dashboard shows stale data | Data is fetched on page mount only. | Refresh the page to re-fetch data. No auto-refresh is implemented for the dashboard. |
