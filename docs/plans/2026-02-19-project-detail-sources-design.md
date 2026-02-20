# Per-Project Source Configuration — Design

> Date: 2026-02-19

## Problem

Source configuration is global-only. Credentials (tokens) live in Settings, and project-specific settings (Jira project key, Linear team, Slack channel) require manually creating `.carto/sources.yaml`. There's no UI to configure which sources a project uses or to manage project-specific settings.

## Design

### Approach: YAML as source of truth, UI as visual editor

`.carto/sources.yaml` in the project root is the canonical config. The UI reads and writes this file via API endpoints. Credentials stay global in Settings — never written to project YAML.

This means CLI, UI, and agents all share the same config. Config travels with the project (git-committable).

### Data Model

**Global (Settings page — already done):** tokens, emails, base URLs.

**Per-project (`.carto/sources.yaml`):** project-specific settings only.

```yaml
sources:
  github:
    owner: divyekant
    repo: carto
  jira:
    url: https://acme.atlassian.net
    project: CARTO
  linear:
    team: ENG
  notion:
    database: abc123-def456
  slack:
    channels: "#eng-carto"
  web:
    urls: https://docs.example.com/api
```

### New API Endpoints

#### GET /api/projects/{name}/sources

Returns parsed sources.yaml merged with credential availability.

```json
{
  "sources": {
    "jira": { "url": "https://acme.atlassian.net", "project": "CARTO" },
    "linear": { "team": "ENG" }
  },
  "credentials": {
    "github_token": true,
    "jira_token": true,
    "jira_email": true,
    "linear_token": false,
    "notion_token": false,
    "slack_token": false
  },
  "auto_detected": {
    "git": true,
    "github": { "owner": "divyekant", "repo": "carto" },
    "local_pdf": true
  }
}
```

- `sources`: entries from `.carto/sources.yaml` (empty object if no file)
- `credentials`: boolean map — which global tokens are configured
- `auto_detected`: sources that will be registered automatically without YAML config

#### PUT /api/projects/{name}/sources

Writes `.carto/sources.yaml`. Request body matches the `sources` field from GET:

```json
{
  "sources": {
    "jira": { "url": "https://acme.atlassian.net", "project": "CARTO" }
  }
}
```

Empty `sources` object (or removing all entries) deletes the YAML file.

### Project Detail Page

New route: `/projects/:name`

Reached by clicking a project card on the Dashboard.

#### Layout

**Header:** Project name, path, last indexed timestamp, file/atom counts.

**Sources card:**
- Lists all available source types in a consistent order: GitHub, Jira, Linear, Notion, Slack, Web, Local PDF
- Auto-detected sources (Git, GitHub, PDF) shown with "Auto" badge — no toggle needed
- Each configurable source has:
  - Enable/disable toggle
  - When enabled: shows project-specific setting fields (project key, team, etc.)
  - Credential status: green "Token configured" badge or amber "Set up in Settings →" link
- Save button at bottom writes `.carto/sources.yaml`

**Index card:**
- Incremental toggle
- Module filter (optional)
- "Index Now" button → triggers POST /api/projects/index with the project path
- Progress bar + log (reuse existing SSE infrastructure)

#### Source Field Definitions

| Source | Fields | Required |
|--------|--------|----------|
| GitHub | owner, repo | Auto-detected from git URL |
| Jira | Base URL, Project key | Both required |
| Linear | Team key | Required |
| Notion | Database ID | Required |
| Slack | Channel ID | Required |
| Web | URLs (comma-separated) | At least one |
| Local PDF | (none) | Auto-detected from docs/ |

### Dashboard Changes

- Project cards become clickable → navigate to `/projects/{name}`
- "Re-index" button moves to project detail page (remove from card)

### What This Does NOT Change

- Settings page stays as-is (global credentials)
- `.carto/sources.yaml` format unchanged
- `BuildRegistry()` unchanged — it already merges YAML + credentials
- CLI unchanged — reads same YAML file
- Index page stays for quick one-off indexing of new projects

## Non-Goals

- Per-project LLM model overrides (use global)
- Index history/log viewer (future)
- Source health checks / test connection per-source (future)
