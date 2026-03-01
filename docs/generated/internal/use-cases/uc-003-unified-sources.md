---
id: uc-003
type: use-case
audience: internal
topic: Configuring External Sources
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Use Case: Configuring External Sources

## Trigger

A user configures external sources to enrich the codebase index with contextual signals from issue trackers, documentation systems, messaging platforms, or other data sources.

## Preconditions

1. Carto is installed and the basic indexing pipeline works (LLM key set, Memories running).
2. Credentials for the desired external services are available.
3. The user knows the relevant project identifiers (repo name, Jira project key, Slack channel ID, etc.).

## Primary Flow: Git-Only (Default)

### Step 1: No Configuration Needed

Git is the default source. It operates on the local repository without credentials. When the pipeline reaches Phase 3, it extracts commits, churn, and ownership data automatically.

### Step 2: Run Indexing

```bash
carto index /path/to/project
```

Phase 3 produces history signals from git. No external source signals are collected.

## Variation: With GitHub Issues and PRs

### Step 1: Set GitHub Credentials

```bash
export GITHUB_TOKEN="ghp_..."
```

### Step 2: Configure Per-Project Source (Optional)

Create `.carto/sources.yaml` in the project:

```yaml
sources:
  github:
    repo: "myorg/myrepo"
    include_issues: true
    include_prs: true
    max_results: 50
```

If `sources.yaml` is not provided, the GitHub source infers the repository from the git remote URL.

### Step 3: Run Indexing

```bash
carto index /path/to/project
```

During Phase 3, `BuildRegistry()` detects `GITHUB_TOKEN` and instantiates the GitHub source. `FetchSignals()` calls the GitHub API to retrieve issues and pull requests. These signals are stored alongside the code atoms and used during deep analysis.

### Step 4: Verify

Check the pipeline output for signal counts. The summary should show a non-zero count for GitHub signals.

## Variation: With Jira and Linear

### Step 1: Set Credentials

```bash
export JIRA_URL="https://company.atlassian.net"
export JIRA_TOKEN="ATATT..."
export JIRA_EMAIL="user@company.com"
export LINEAR_TOKEN="lin_api_..."
```

### Step 2: Configure Per-Project Sources

```yaml
sources:
  jira:
    project_key: "PROJ"
    max_results: 100
  linear:
    team: "backend"
    max_results: 50
```

### Step 3: Run Indexing

Both Jira and Linear sources are instantiated. They fetch issues concurrently during Phase 3. The pipeline combines signals from both sources with git history data.

## Variation: With Notion Documentation

### Step 1: Set Credentials

```bash
export NOTION_TOKEN="ntn_..."
```

### Step 2: Configure Per-Project Source

```yaml
sources:
  notion:
    database_id: "abc123def456"
```

Or specify individual page IDs:

```yaml
sources:
  notion:
    page_ids:
      - "page-id-1"
      - "page-id-2"
```

### Step 3: Ensure Integration Access

In Notion, the integration must be shared with the target database or pages. This is done through Notion's "Share" dialog on each page/database.

### Step 4: Run Indexing

The Notion source fetches page content, converts Notion blocks to plain text, and provides them as documentation signals.

## Variation: With Slack Context

### Step 1: Set Credentials

```bash
export SLACK_TOKEN="xoxb-..."
```

The bot token must have `channels:history` scope. The bot must be invited to the target channel.

### Step 2: Configure Per-Project Source

```yaml
sources:
  slack:
    channel: "C0123456789"
    max_results: 200
```

Use the channel ID (starts with `C`), not the channel name.

### Step 3: Run Indexing

The Slack source fetches recent messages from the configured channel and provides them as context signals.

## Variation: With PDF Specifications

### Step 1: No Credentials Needed

PDF sources read local files. No API tokens required.

### Step 2: Configure Per-Project Source

```yaml
sources:
  pdf:
    paths:
      - "./docs/specs/*.pdf"
      - "./design/requirements.pdf"
```

Paths are relative to the project root. Glob patterns are supported.

### Step 3: Run Indexing

The PDF source extracts text from matching files and provides it as documentation signals.

## Variation: With Web Documentation

### Step 1: No Credentials Needed

Web sources fetch public URLs. No API tokens required.

### Step 2: Configure Per-Project Source

```yaml
sources:
  web:
    urls:
      - "https://docs.example.com/api-reference"
      - "https://wiki.example.com/architecture"
```

### Step 3: Run Indexing

The web source fetches each URL, extracts text content, and provides it as documentation signals.

## Variation: All Sources Combined

All sources can be enabled simultaneously. The `sources.yaml` file lists all desired sources, and credentials are set via environment variables. During Phase 3, all sources run concurrently:

```yaml
sources:
  github:
    repo: "myorg/myrepo"
  jira:
    project_key: "PROJ"
  linear:
    team: "backend"
  notion:
    database_id: "abc123"
  slack:
    channel: "C0123456789"
  pdf:
    paths: ["./docs/*.pdf"]
  web:
    urls: ["https://docs.example.com"]
```

## Edge Cases

| Scenario | Behavior |
|---|---|
| Credential set but per-project config missing | Source uses defaults. GitHub infers repo from git remote. Jira/Linear may fail without a project key/team. |
| Credential missing for a configured source | Source is not instantiated. No error -- it is silently skipped. |
| Source API returns an error | Error is logged, collected in `Result.Errors`, pipeline continues with other sources. |
| `sources.yaml` syntax error | File parsing fails. All sources fall back to global-only config. Error is logged. |
| Very large number of results from a source | Most sources have `max_results` caps. If not configured, the source applies its own default limit. |

## Data Impact

**Written:**
- Memories: `carto/{project}/{module}/layer:signals` -- signal entries from all active sources, tagged with source name.

**Not Written:**
- Raw API responses are not stored. Only the processed `Signal` structs are persisted.
- Credentials are never stored in Memories.

## Post-Conditions

1. Signals from all configured and available sources are stored in Memories.
2. The deep analysis phase (Phase 4) has access to external context for richer analysis.
3. Any source failures are recorded in `Result.Errors` without blocking the pipeline.
