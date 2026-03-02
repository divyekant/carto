# UI Modernization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform the Carto web UI from a sparse prototype into a data-rich, polished developer tool with dashboard stats, card-grouped sections, larger typography, and an always-expanded sidebar.

**Architecture:** 8 tasks, frontend-first. Tasks 1-3 are foundational (typography, cards, sidebar). Task 4 adds backend stats API. Tasks 5-8 redesign each page. Each task is independently committable. The frontend uses React 19 + Tailwind 4 + shadcn/ui.

**Tech Stack:** React 19, React Router 7, Tailwind CSS 4, shadcn/ui, Vite 7, Go (backend)

**Design Reference:** `docs/plans/2026-03-01-ui-modernization-design.md` (approved)

---

### Task 1: Global Typography & Spacing

Increase font sizes and spacing across the app. This is the single highest-impact change — fixes "too small text" and "too much whitespace" in one shot.

**Files:**
- Modify: `go/web/src/components/Layout.tsx:118` (content area padding)
- Modify: `go/web/src/pages/Dashboard.tsx:80-94` (page title)
- Modify: `go/web/src/pages/IndexRun.tsx:220-224` (page title)
- Modify: `go/web/src/pages/Query.tsx:80-82` (page title)
- Modify: `go/web/src/pages/Settings.tsx:395-399` (page title)

**Step 1: Update Layout content area padding**

In `go/web/src/components/Layout.tsx`, change the main content area (line 118) from:
```tsx
<main className="flex-1 overflow-y-auto p-3 pt-14 md:p-5 md:pt-5">
```
to:
```tsx
<main className="flex-1 overflow-y-auto p-4 pt-16 md:p-8 md:pt-8">
```

**Step 2: Update page titles to text-2xl font-bold**

In each page component, find the page title `<h2>` and change from `text-lg font-semibold` to `text-2xl font-bold`. Also update subtitle/metadata from `text-xs` to `text-sm`, and section labels from `text-xs` to `text-sm`.

Dashboard.tsx (line ~80):
```tsx
<h2 className="text-2xl font-bold">Dashboard</h2>
<span className="text-sm text-muted-foreground">{projects.length} projects</span>
```

IndexRun.tsx (line ~220):
```tsx
<h2 className="text-2xl font-bold">Index Project</h2>
```

Query.tsx (line ~80):
```tsx
<h2 className="text-2xl font-bold">Query</h2>
```

Settings.tsx (line ~395):
```tsx
<h2 className="text-2xl font-bold">Settings</h2>
```

**Step 3: Update body text sizes throughout**

Across all pages, do a pass to upgrade:
- Form labels from `text-xs` to `text-sm font-medium`
- Help text/descriptions from `text-xs` to `text-sm`
- Table cells from `text-sm` to `text-base` for primary content
- Table headers from `text-xs` to `text-sm font-medium`

**Step 4: Verify visually**

Run: `cd go/web && npm run dev`
Check all 4 pages in browser. Titles should be noticeably larger. Content text should be readable without squinting.

**Step 5: Commit**

```bash
git add go/web/src/
git commit -m "style: increase typography scale and content padding across all pages"
```

---

### Task 2: Card Section Component

Create a reusable `Section` wrapper component that provides card-style grouping with a title. This will be used by every page redesign.

**Files:**
- Create: `go/web/src/components/Section.tsx`

**Step 1: Create the Section component**

```tsx
import { cn } from '@/lib/utils';

interface SectionProps {
  title?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function Section({ title, action, children, className }: SectionProps) {
  return (
    <div className={cn('rounded-lg border border-border/50 bg-card p-5', className)}>
      {(title || action) && (
        <div className="mb-4 flex items-center justify-between">
          {title && <h3 className="text-lg font-semibold">{title}</h3>}
          {action}
        </div>
      )}
      {children}
    </div>
  );
}
```

**Step 2: Create a StatCard component for dashboard stats**

```tsx
interface StatCardProps {
  label: string;
  value: string | number;
  icon?: React.ReactNode;
  status?: 'ok' | 'error' | 'unknown';
  detail?: string;
}

export function StatCard({ label, value, icon, status, detail }: StatCardProps) {
  return (
    <div className="rounded-lg border border-border/50 bg-card p-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        {icon}
        {label}
      </div>
      <div className="mt-1 text-2xl font-bold">{value}</div>
      {detail && <div className="mt-0.5 text-sm text-muted-foreground">{detail}</div>}
      {status && (
        <div className="mt-1 flex items-center gap-1.5 text-sm">
          <span className={cn(
            'inline-block h-2 w-2 rounded-full',
            status === 'ok' && 'bg-emerald-500',
            status === 'error' && 'bg-red-500',
            status === 'unknown' && 'bg-muted-foreground/50',
          )} />
          {status === 'ok' ? 'Connected' : status === 'error' ? 'Disconnected' : 'Unknown'}
        </div>
      )}
    </div>
  );
}
```

**Step 3: Verify component renders**

Import and render a test `<Section title="Test">Hello</Section>` in Dashboard temporarily. Check it renders a bordered card with title.

**Step 4: Remove test usage and commit**

```bash
git add go/web/src/components/Section.tsx
git commit -m "feat(web): add Section and StatCard reusable components"
```

---

### Task 3: Sidebar Redesign — Always Expanded with Status

Replace the hover-expand icon rail with a persistent 224px sidebar showing labels, server status, and version.

**Files:**
- Modify: `go/web/src/components/Layout.tsx` (lines 6-123, complete rewrite of sidebar)

**Step 1: Rewrite the sidebar section in Layout.tsx**

Replace the entire sidebar `<aside>` (lines 75-115) with an always-expanded sidebar:

Key changes:
- Width: `w-56` fixed (was `w-12 hover:w-48`)
- Remove `group/sidebar` hover logic
- Nav items always show icon + label (remove `opacity-0 group-hover/sidebar:opacity-100`)
- Add server status section between nav and footer
- Add version in footer next to theme toggle
- Fetch `/api/health` on mount to show server status

The sidebar needs state for health. Add to the component:
```tsx
const [health, setHealth] = useState<{ memories_healthy: boolean } | null>(null);

useEffect(() => {
  fetch('/api/health').then(r => r.json()).then(setHealth).catch(() => {});
}, []);
```

Sidebar structure:
```tsx
<aside className="hidden md:flex w-56 flex-col border-r border-border/50 bg-sidebar">
  {/* Logo */}
  <div className="flex h-14 items-center gap-2 px-4">
    <span className="text-lg font-bold text-primary">C</span>
    <span className="text-lg font-bold">Carto</span>
  </div>

  {/* Nav items */}
  <nav className="flex-1 space-y-1 px-3 py-2">
    {navItems.map(item => (
      <NavLink key={item.to} to={item.to}
        className={({ isActive }) => cn(
          'flex items-center gap-3 rounded-md px-3 py-2 text-base transition-colors',
          isActive ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground hover:bg-muted hover:text-foreground'
        )}>
        {item.icon}
        {item.label}
      </NavLink>
    ))}
  </nav>

  {/* Server status */}
  <div className="border-t border-border/50 px-4 py-3 space-y-2">
    <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Server</div>
    <div className="flex items-center gap-2 text-sm">
      <span className={cn('h-2 w-2 rounded-full', health?.memories_healthy ? 'bg-emerald-500' : 'bg-red-500')} />
      Memories: {health?.memories_healthy ? 'OK' : 'Down'}
    </div>
  </div>

  {/* Footer */}
  <div className="flex items-center justify-between border-t border-border/50 px-4 py-3">
    <span className="text-xs text-muted-foreground">v1.0.0</span>
    <ThemeToggle />
  </div>
</aside>
```

**Step 2: Update mobile header to match**

Keep existing hamburger + overlay approach for mobile, but update the slide-out menu to use same styling as desktop sidebar.

**Step 3: Update main content area**

Since sidebar is now always w-56, the main content flows naturally with `flex-1`. No changes needed beyond what was done in Task 1.

**Step 4: Verify visually**

Run dev server. Check:
- Desktop: sidebar always visible with labels, status section, version
- Mobile: hamburger opens full sidebar overlay
- Theme toggle still works
- Active nav item highlighted with primary/10 background

**Step 5: Commit**

```bash
git add go/web/src/components/Layout.tsx
git commit -m "feat(web): redesign sidebar to always-expanded with server status"
```

---

### Task 4: Backend Stats API Endpoint

Add `GET /api/stats` returning aggregate project data + recent activity for the Dashboard.

**Files:**
- Modify: `go/internal/server/handlers.go` (add handler)
- Modify: `go/internal/server/routes.go` (add route)
- Create: `go/internal/server/handlers_stats_test.go` (test)

**Step 1: Write the failing test**

Create `go/internal/server/handlers_stats_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleStats(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	srv.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp statsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.TotalProjects < 0 {
		t.Error("expected non-negative total_projects")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/server/ -run TestHandleStats -v`
Expected: FAIL — `handleStats` not defined.

**Step 3: Implement the stats handler**

In `go/internal/server/handlers.go`, add:

```go
type statsResponse struct {
	TotalProjects int             `json:"total_projects"`
	TotalAtoms    int             `json:"total_atoms"`
	TotalFiles    int             `json:"total_files"`
	Memories      memoriesStatus  `json:"memories"`
	RecentRuns    []runStatusItem `json:"recent_runs"`
}

type memoriesStatus struct {
	Healthy bool   `json:"healthy"`
	Latency string `json:"latency,omitempty"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	projects := s.listProjectInfos()

	totalFiles := 0
	totalAtoms := 0
	for _, p := range projects {
		totalFiles += p.FileCount
		// Atom count from manifest if available
	}

	// Check memories health
	memHealth := memoriesStatus{Healthy: false}
	if s.memoriesClient != nil {
		start := time.Now()
		if err := s.memoriesClient.Ping(r.Context()); err == nil {
			memHealth.Healthy = true
			memHealth.Latency = fmt.Sprintf("%dms", time.Since(start).Milliseconds())
		}
	}

	// Get recent runs from active/completed runs
	runs := s.getRunStatuses()

	writeJSON(w, statsResponse{
		TotalProjects: len(projects),
		TotalAtoms:    totalAtoms,
		TotalFiles:    totalFiles,
		Memories:      memHealth,
		RecentRuns:    runs,
	})
}
```

Note: Adapt to actual server struct fields. The `listProjectInfos()` and `getRunStatuses()` functions may already exist — reuse `handleListProjects` and `handleListRuns` logic.

**Step 4: Register the route**

In `go/internal/server/routes.go`, add after line 27:
```go
mux.HandleFunc("GET /api/stats", s.handleStats)
```

**Step 5: Run test to verify it passes**

Run: `cd go && go test ./internal/server/ -run TestHandleStats -v`
Expected: PASS

**Step 6: Commit**

```bash
git add go/internal/server/
git commit -m "feat(api): add GET /api/stats endpoint for dashboard aggregate data"
```

---

### Task 5: Dashboard Page Redesign

Replace the sparse project list with stats cards + enriched table + activity feed.

**Files:**
- Modify: `go/web/src/pages/Dashboard.tsx` (lines 53-142, major rewrite)

**Step 1: Add stats fetching**

Add a new state and fetch for `/api/stats`:
```tsx
interface Stats {
  total_projects: number;
  total_atoms: number;
  total_files: number;
  memories: { healthy: boolean; latency?: string };
  recent_runs: Array<{
    project: string;
    status: string;
    result?: { atoms?: number };
    error?: string;
    started_at?: string;
  }>;
}

const [stats, setStats] = useState<Stats | null>(null);
```

Fetch in the existing useEffect alongside projects.

**Step 2: Build the stats bar**

Below the page header, add a 4-column grid of StatCard components:

```tsx
<div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
  <StatCard label="Projects" value={stats?.total_projects ?? 0} icon={<FolderIcon />} />
  <StatCard label="Atoms" value={stats?.total_atoms ?? 0} icon={<AtomIcon />} />
  <StatCard label="Files" value={stats?.total_files ?? 0} icon={<FileIcon />} />
  <StatCard
    label="Memories"
    value={stats?.memories.latency ?? '—'}
    status={stats?.memories.healthy ? 'ok' : 'error'}
  />
</div>
```

**Step 3: Enrich the projects table**

Wrap in `<Section title="Projects">`. Add columns for Modules and Atoms if available from stats. Keep existing columns (Name, Path, Files, Indexed, Status). Upgrade text sizes per Task 1.

**Step 4: Add recent activity feed**

Below the table, add:
```tsx
<Section title="Recent Activity">
  {stats?.recent_runs.length === 0 && (
    <p className="text-sm text-muted-foreground">No recent activity.</p>
  )}
  <div className="space-y-2">
    {stats?.recent_runs.map((run, i) => (
      <div key={i} className="flex items-center justify-between rounded-md border border-border/30 px-4 py-2.5">
        <div className="flex items-center gap-3">
          <span className={cn('h-2 w-2 rounded-full',
            run.status === 'complete' ? 'bg-emerald-500' :
            run.status === 'error' ? 'bg-red-500' : 'bg-yellow-500'
          )} />
          <span className="font-medium">{run.project}</span>
          <span className="text-sm text-muted-foreground">
            {run.status === 'complete' ? `${run.result?.atoms ?? 0} atoms` : run.error ?? run.status}
          </span>
        </div>
        <span className="text-sm text-muted-foreground">{getTimeAgo(run.started_at)}</span>
      </div>
    ))}
  </div>
</Section>
```

**Step 5: Improve empty state**

Replace bare "No indexed projects yet." with a card:
```tsx
<Section>
  <div className="flex flex-col items-center gap-4 py-12 text-center">
    <div className="rounded-full bg-primary/10 p-4">
      <FolderIcon className="h-8 w-8 text-primary" />
    </div>
    <div>
      <h3 className="text-lg font-semibold">No indexed projects yet</h3>
      <p className="mt-1 text-sm text-muted-foreground">Get started by indexing your first codebase</p>
    </div>
    <div className="flex flex-col gap-2 text-sm text-muted-foreground">
      <span>1. Configure your LLM provider in Settings</span>
      <span>2. Index your first project</span>
    </div>
    <Button onClick={() => navigate('/index')}>Index Your First Project</Button>
  </div>
</Section>
```

**Step 6: Verify visually**

Run dev server. Check dashboard with and without projects. Stats cards should show across the top. Table should be in a card section. Activity feed should fill the bottom.

**Step 7: Commit**

```bash
git add go/web/src/pages/Dashboard.tsx
git commit -m "feat(web): redesign dashboard with stats cards, enriched table, activity feed"
```

---

### Task 6: Settings Page — Card Sections

Reorganize the flat 2-column Settings layout into 4 stacked card sections.

**Files:**
- Modify: `go/web/src/pages/Settings.tsx` (lines 395-672, restructure into card sections)

**Step 1: Restructure the JSX into 4 Section cards**

Replace the 2-column grid layout with stacked sections:

```tsx
<div className="space-y-6">
  {/* Header with sticky save */}
  <div className="flex items-center justify-between">
    <h2 className="text-2xl font-bold">Settings</h2>
    <Button onClick={save} disabled={saving}>{saving ? 'Saving...' : 'Save Settings'}</Button>
  </div>

  {/* Section 1: LLM Provider */}
  <Section title="LLM Provider">
    {/* Provider select + API key row */}
    {/* Fast/Deep model selects row */}
  </Section>

  {/* Section 2: Performance */}
  <Section title="Performance">
    {/* Max Concurrent, Fast Max Tokens, Deep Max Tokens — 3-column grid */}
  </Section>

  {/* Section 3: Memories Server */}
  <Section title="Memories Server">
    {/* URL + Key row */}
    {/* Connection status inline + Test button */}
  </Section>

  {/* Section 4: Integrations */}
  <Section title="Integrations">
    {/* Each integration as a row with label, input, status dot */}
  </Section>
</div>
```

**Step 2: Move existing form fields into their new sections**

This is a reorganization, not rewrite. Move the existing JSX blocks:
- Lines 401-535 (LLM provider fields) → Section 1 + Section 2
- Lines 537-560 (Memories URL/Key/Test) → Section 3
- Lines 562-663 (GitHub, Jira, Linear, Notion, Slack) → Section 4

**Step 3: Add connection status indicators to Integrations**

For each integration field, add a status dot:
```tsx
<div className="flex items-center gap-3">
  <span className={cn('h-2 w-2 rounded-full',
    config.github_token ? 'bg-emerald-500' : 'bg-muted-foreground/30'
  )} />
  <Label className="w-24 text-sm font-medium">GitHub</Label>
  <Input ... className="flex-1" />
</div>
```

**Step 4: Verify visually**

Run dev server. Settings should show 4 distinct card sections stacked vertically. Save button top-right. Each integration should show a green/gray dot.

**Step 5: Commit**

```bash
git add go/web/src/pages/Settings.tsx
git commit -m "feat(web): reorganize settings into 4 grouped card sections"
```

---

### Task 7: Index Page — Card Wrapping + Recent Runs

Wrap the form and progress areas in cards, add recent runs panel.

**Files:**
- Modify: `go/web/src/pages/IndexRun.tsx` (wrap in Section cards, add recent runs)

**Step 1: Wrap the source form in a Section**

Wrap the tab toggle + form inputs (lines 225-310) in `<Section title="Source">`.

**Step 2: Wrap progress/logs in a Section**

Wrap the running-state progress bar + logs panel (lines 314-411) in `<Section title="Progress">` when state is running/complete/error.

**Step 3: Add Recent Runs panel**

Below the form, fetch and display recent runs:

```tsx
const [recentRuns, setRecentRuns] = useState<any[]>([]);

useEffect(() => {
  fetch('/api/projects/runs').then(r => r.json()).then(setRecentRuns).catch(() => {});
}, []);
```

```tsx
{pageState === 'idle' && recentRuns.length > 0 && (
  <Section title="Recent Runs">
    <div className="space-y-2">
      {recentRuns.slice(0, 5).map((run, i) => (
        <div key={i} className="flex items-center justify-between rounded-md border border-border/30 px-4 py-2.5">
          <div className="flex items-center gap-3">
            <span className={cn('h-2 w-2 rounded-full',
              run.status === 'complete' ? 'bg-emerald-500' :
              run.status === 'error' ? 'bg-red-500' : 'bg-yellow-500'
            )} />
            <span className="font-medium">{run.project}</span>
          </div>
          <span className="text-sm text-muted-foreground">{run.status}</span>
        </div>
      ))}
    </div>
  </Section>
)}
```

**Step 4: Verify visually**

Check: form in a card, progress in a card, recent runs visible when idle.

**Step 5: Commit**

```bash
git add go/web/src/pages/IndexRun.tsx
git commit -m "feat(web): wrap index page in card sections, add recent runs panel"
```

---

### Task 8: Query Page — Result Cards + Quick Queries

Redesign query results and add suggestion panel.

**Files:**
- Modify: `go/web/src/pages/Query.tsx` (wrap in sections, add quick queries)
- Modify: `go/web/src/components/QueryResult.tsx` (card styling + score badge)

**Step 1: Wrap search controls in a Section**

Wrap the search bar and controls (lines 84-146) in `<Section title="Search">`.

Reorganize: project selector + tier on row 1, search input (larger) on row 2.

**Step 2: Improve result cards**

In `QueryResult.tsx`, update the score display to use a colored badge:
```tsx
<span className={cn(
  'inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium',
  score > 0.8 ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400' :
  score > 0.5 ? 'bg-yellow-500/15 text-yellow-700 dark:text-yellow-400' :
  'bg-muted text-muted-foreground'
)}>
  {score.toFixed(2)}
</span>
```

**Step 3: Add Quick Queries panel**

When no search has been performed yet, show suggestion chips:

```tsx
const QUICK_QUERIES = [
  'authentication flow', 'error handling', 'API endpoints',
  'database schema', 'configuration', 'middleware',
];

{!searched && (
  <Section title="Quick Queries" className="mt-6">
    <p className="mb-3 text-sm text-muted-foreground">Try one of these to get started:</p>
    <div className="flex flex-wrap gap-2">
      {QUICK_QUERIES.map(q => (
        <button key={q} onClick={() => { setText(q); }}
          className="rounded-full border border-border/50 px-3 py-1.5 text-sm hover:bg-muted transition-colors">
          {q}
        </button>
      ))}
    </div>
  </Section>
)}
```

**Step 4: Wrap results in Section**

```tsx
{searched && results.length > 0 && (
  <Section title={`Results (${results.length} matches)`} className="mt-6">
    {/* existing results list */}
  </Section>
)}
```

**Step 5: Verify visually**

Check: search in a card, quick queries appear when idle, results in a card with colored score badges.

**Step 6: Commit**

```bash
git add go/web/src/pages/Query.tsx go/web/src/components/QueryResult.tsx
git commit -m "feat(web): redesign query page with result cards and quick query suggestions"
```

---

## Task Order & Dependencies

```
Task 1 (typography) ─┐
Task 2 (Section comp) ├── Foundation (no deps between 1-3)
Task 3 (sidebar)     ─┘
         │
Task 4 (stats API) ──── Backend (needs test server helper from existing tests)
         │
Task 5 (dashboard) ──── Uses Section, StatCard, stats API
Task 6 (settings)  ──── Uses Section
Task 7 (index page) ─── Uses Section
Task 8 (query page) ─── Uses Section
```

Tasks 1-3 can be done in parallel. Task 4 can be done in parallel with 1-3. Tasks 5-8 depend on Task 2 (Section component) and can be done in parallel with each other.
