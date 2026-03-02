# Carto UI Modernization Design

> **Date:** 2026-03-01
> **Status:** Approved
> **Approach:** Hybrid — Dashboard-first data density + Visual polish

## Goals

- Eliminate empty space by filling pages with useful data
- Upgrade from prototype feel to polished open-source product
- Match data-rich aesthetic (Grafana/Datadog style density)
- Fix undersized typography across the board
- Group related controls into card sections

## Audience

Personal use + CleverTap team + open-source developers.

---

## 1. Global Visual Upgrades

### Typography Scale-Up

| Element | Current | New |
|---------|---------|-----|
| Page titles | `text-lg font-semibold` | `text-2xl font-bold` |
| Section headings | `text-sm` | `text-lg font-semibold` |
| Body text | `text-sm` | `text-base` |
| Secondary/metadata | `text-xs` | `text-sm` |

### Spacing & Density

- Main content padding: `p-6 md:p-8` (from `p-3 md:p-5`)
- Section gaps: `gap-6` (from `space-y-3`)
- Card internal padding: `p-5`

### Card System

Every content section wrapped in a card: `bg-card border border-border/50 rounded-lg`.

### Color Refinements

- Warmer dark mode background (charcoal, not pure black)
- Better light mode contrast
- Accent color (cyan) used for status indicators, active states, key CTAs only
- Muted background tints for section differentiation

### Micro-Interactions

- Skeleton loading states for async data
- Hover transitions: `transition-colors duration-150`
- Progress bar animations

---

## 2. Dashboard Page

### Layout: 3 zones

**Zone 1 — Stats Bar (4 cards across top):**
- Total Projects count
- Total Atoms count
- Total Files count
- Memories Server health (connected/disconnected + latency)

**Zone 2 — Projects Table (enriched):**

| Column | Description |
|--------|-------------|
| Name | Project name, clickable to detail |
| Modules | Module count |
| Files | File count |
| Atoms | Atom count |
| Status | Badge: running/complete/error |
| Last Run | Relative time |

**Zone 3 — Recent Activity Feed:**
Chronological list of indexing events showing project name, event type (full/incremental/failed), and relative time.

**Empty State:**
Illustrated onboarding card with steps:
1. Configure your LLM provider in Settings
2. Index your first project

### Backend Required

New `GET /api/stats` endpoint returning:
- Aggregate counts (projects, atoms, files)
- Memories server health (status, latency)
- Recent activity list (last 10 indexing events)

---

## 3. Index Project Page

### Layout: Form Card + Progress Split + Recent Runs

**Source Card:** Form inputs wrapped in a card with "Source" header. Local Path / Git URL tabs, path input, module filter, incremental toggle, Start button.

**Progress/Log Split:** Side-by-side layout (already partially exists). Left: phase checklist with status icons (checkmark/spinner/circle) + progress bar. Right: scrolling log output.

**Recent Runs Panel:** Below progress area. Shows last 5 indexing runs across all projects with: project name, mode (full/incremental), result (success/failed), atom count, relative time.

### Backend Required

`GET /api/runs/recent` endpoint (or reuse activity feed from stats endpoint).

---

## 4. Query Page

### Layout: Search Card + Result Cards + Quick Queries

**Search Card:** Project selector and tier on one row, search input below (larger, prominent). Count control inline.

**Results in Cards:** Each result in a card with:
- Score as colored badge (green >0.8, yellow >0.5, gray otherwise)
- File path in monospace
- Summary text
- Expandable preview

**Quick Queries Panel:** Shown when no results displayed. Example queries: "authentication flow", "error handling", "API endpoints", "database schema". Clickable to populate search.

### Backend Required

None — pure frontend changes.

---

## 5. Settings Page

### Layout: 4 stacked card sections (single column)

**Card 1 — LLM Provider:**
Provider dropdown, API key, Fast Model, Deep Model. Help text per field.

**Card 2 — Performance:**
Max Concurrent, Fast Max Tokens, Deep Max Tokens. Inline descriptions.

**Card 3 — Memories Server:**
URL, Key, live connection status (green dot + latency), Test button.

**Card 4 — Integrations:**
Table-like layout: GitHub, Jira, Linear, Notion, Slack. Each row: label, input, connection status indicator (green dot = configured, gray = not set).

**Save Button:** Top-right, sticky position.

### Backend Required

None — pure frontend reorganization of existing fields.

---

## 6. Sidebar & Layout

### Always-Expanded Sidebar (desktop)

- Width: `w-56` (224px), always visible on `md+` screens
- Mobile: collapsible via hamburger (existing behavior)
- Nav items with `text-base`, active state uses subtle background fill

### Server Status Section

Below nav items, above footer:
- Memories connection status (green/red dot + "Connected"/"Disconnected")
- LLM provider name

### Footer

Version number (`v1.0.0`) + theme toggle side by side.

---

## Summary of Backend Work

| Endpoint | Purpose |
|----------|---------|
| `GET /api/stats` | Aggregate counts, server health, recent activity |
| `GET /api/runs/recent` | Last N indexing runs (may merge with stats) |

Both endpoints read from existing Memories storage — no new data models needed.

---

## Tech Stack (unchanged)

- React 19 + React Router 7
- Tailwind CSS 4 + shadcn/ui
- Vite 7
- Sonner for toasts
- lucide-react for icons
