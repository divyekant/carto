# CLI/API Parity + Dense UI Redesign

## Problem

1. The CLI and API don't cover everything — sources, batch ops, project CRUD, and config are only partially exposed. No machine-readable `--json` output. No SDK/library mode.
2. The UI is too spacious — card-heavy, single-column layouts, excessive padding. Should feel like a dense operational dashboard, not a SaaS landing page.

## Stream 1: CLI/API — Full CRUD + `--json` + SDK

**Approach:** CLI-first, API follows. Cobra subcommands for every operation. `--json` flag on all commands. Internal packages stay importable as a library.

### New CLI Commands

| Command | Description | API Endpoint |
|---------|-------------|-------------|
| `carto projects list` | List indexed projects | `GET /api/projects` (exists) |
| `carto projects show <name>` | Detailed project info | `GET /api/projects/{name}` (new) |
| `carto projects delete <name>` | Remove project index | `DELETE /api/projects/{name}` (new) |
| `carto sources list <project>` | Show configured sources | `GET /api/projects/{name}/sources` (exists) |
| `carto sources set <project> <type> --key=val` | Add/update a source | `PUT /api/projects/{name}/sources` (exists) |
| `carto sources rm <project> <type>` | Remove a source | `PUT /api/projects/{name}/sources` (exists, send without key) |
| `carto config get [key]` | Show config (all or one) | `GET /api/config` (exists) |
| `carto config set <key> <value>` | Set a config value | `PATCH /api/config` (exists) |
| `carto index --all` | Re-index all projects | `POST /api/projects/index-all` (new) |
| `carto index --changed` | Re-index modified projects | `POST /api/projects/index-all?changed=true` (new) |

### Global Flags

- `--json` — machine-readable JSON output on every command
- `--quiet` / `-q` — suppress progress spinners, only output result

### New API Endpoints

- `GET /api/projects/{name}` — single project detail (files, sources, last indexed, manifest summary)
- `DELETE /api/projects/{name}` — remove `.carto/` directory for a project
- `POST /api/projects/index-all` — batch index all (or `?changed=true` for incremental)

### SDK / Library Mode

Thin public package at `pkg/carto`:

```go
import "github.com/divyekant/carto/pkg/carto"

result, err := carto.Index("/path/to/project", carto.IndexOptions{
    Incremental: true,
    Module:      "go",
})

results, err := carto.Query("how does auth work", carto.QueryOptions{
    Project: "myapp",
    Tier:    "standard",
    K:       10,
})

sources, err := carto.Sources("myapp")
```

Wraps existing internal packages. Minimal surface. Stable API contract.

---

## Stream 2: UI — Dense Dashboard Redesign

**Approach:** Same routes, same React + shadcn/ui + Tailwind stack. Compact in-place redesign — tighter spacing, data tables, multi-column layouts.

### Global Changes

- **Sidebar:** Icon-only by default (40px), expands on hover. Reclaims ~180px.
- **Kill width constraints:** Remove all `max-w-2xl` / `max-w-lg`. Content fills available width.
- **Typography:** Base text `text-xs`/`text-sm`. Headings `text-base`/`text-lg`.
- **Spacing:** Card padding `py-2 px-3`. Section gaps `gap-3` not `gap-6`.

### Page Redesigns

**Dashboard:**
- Replace card grid with data table (shadcn Table). One row per project.
- Columns: Name, Path (truncated), Files, Sources (count), Last Indexed, Status, Actions.
- Sortable columns. Clickable rows → project detail.
- Header bar: Memories health badge + "Index New" button.

**Index:**
- Compact single-row form: path/URL input + options (incremental, module) inline.
- Below: progress bar left, scrolling log right (side by side).
- No full-page cards wrapping single inputs.

**Project Detail:**
- Two-column layout. Left: Sources (compact toggles with inline fields). Right: Index controls + last run summary.
- Everything visible on one screen.

**Query:**
- Search bar pinned at top with filters inline (project, tier, count on same row).
- Results as compact list rows — source left, truncated text right, score as thin bar.
- Click to expand. No cards per result.

**Settings:**
- Two-column grid. Left: LLM config (provider, models, concurrency). Right: Connections (Memories, integrations).
- All compact inputs. One screen, no scrolling.

### New shadcn Components Needed

- `Table` (for Dashboard data table)
- `Switch` (replace custom toggle switches in SourcesEditor)
- `Tooltip` (for redacted keys, status hints)

### Components to Retire

- `ProjectCard` — replaced by table rows
- Custom toggle button in SourcesEditor — replaced by shadcn Switch
