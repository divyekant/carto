---
id: fh-003
type: feature-handoff
audience: internal
topic: Unified Sources
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Handoff: Unified Sources

## What It Does

The unified sources system provides a pluggable architecture for collecting contextual signals from external services during pipeline Phase 3. It replaces the older separate Signals and Knowledge registries with a single `Source` interface and a registry pattern.

Each source fetches data relevant to the codebase being indexed -- issues, pull requests, documentation pages, chat messages, PDF content, or web page text. This data enriches the semantic index with context that is not available from the source code alone.

## How It Works

### Source Interface

Every source implements a common interface:

```
Name() string                    -- Returns the source identifier (e.g., "github", "jira")
Configure(config SourceConfig)   -- Applies per-project or global configuration
FetchSignals(ctx context.Context, project string) ([]Signal, error)
                                 -- Fetches signals for a given project
```

### Registry Pattern

`BuildRegistry()` in `registry.go` creates the set of active sources based on available configuration:

1. Reads environment variables for credentials.
2. Reads per-project source configuration from `sources.yaml` if present.
3. Instantiates only the sources that have valid credentials configured.
4. Returns a `SourceRegistry` that the pipeline uses in Phase 3.

Sources without credentials are silently skipped -- they are not instantiated and produce no errors.

### Concurrent Fetching

During Phase 3, the pipeline calls `FetchAll()` on the registry. This runs each source's `FetchSignals()` concurrently. Individual source failures are caught, logged, and collected in the pipeline's error list. Other sources continue running.

### Per-Project Configuration

The `sources.yaml` file (placed in the project root or `.carto/` directory) allows per-project source configuration:

```yaml
sources:
  github:
    repo: "owner/repo"
    include_prs: true
    include_issues: true
  jira:
    project_key: "PROJ"
    max_results: 100
  linear:
    team: "engineering"
  notion:
    database_id: "abc123"
  slack:
    channel: "C0123456789"
  pdf:
    paths:
      - "./docs/specs/*.pdf"
  web:
    urls:
      - "https://docs.example.com/api"
```

## Source Types

### Git

**Component:** `git.go`

Built-in source. Always available (no credentials needed). Extracts:

- Recent commits (messages, authors, timestamps)
- Pull request references parsed from commit messages
- File change frequency (churn)
- Code ownership (last modifier per file)

**Config:** None required. Operates on the target repository directly.

### GitHub

**Component:** `github.go`

Fetches issues and pull requests from the GitHub API.

**Provides:**
- Open and recently closed issues (titles, descriptions, labels, assignees)
- Open and recently merged pull requests (titles, descriptions, review status)

**Config:**

| Variable | Description |
|---|---|
| `GITHUB_TOKEN` | Personal access token or fine-grained token with `repo` scope |

**Per-project:** `repo` (owner/repo), `include_prs`, `include_issues`, `max_results`

### Jira

**Component:** `jira.go`

Fetches issues from a Jira instance via REST API.

**Provides:**
- Issues matching the configured project key (titles, descriptions, status, priority, assignee)

**Config:**

| Variable | Description |
|---|---|
| `JIRA_URL` | Jira instance URL (e.g., `https://company.atlassian.net`) |
| `JIRA_TOKEN` | API token for authentication |
| `JIRA_EMAIL` | Email associated with the API token |

**Per-project:** `project_key`, `max_results`, `jql` (custom JQL filter)

### Linear

**Component:** `linear.go`

Fetches issues from Linear via GraphQL API.

**Provides:**
- Issues for the configured team (titles, descriptions, status, priority, assignee)

**Config:**

| Variable | Description |
|---|---|
| `LINEAR_TOKEN` | Linear API key |

**Per-project:** `team`, `max_results`

### Notion

**Component:** `notion.go`

Fetches pages from a Notion database or workspace.

**Provides:**
- Page titles and content (converted from Notion blocks to plain text)

**Config:**

| Variable | Description |
|---|---|
| `NOTION_TOKEN` | Notion integration token |

**Per-project:** `database_id`, `page_ids`, `max_results`

### Slack

**Component:** `slack.go`

Fetches messages from a Slack channel.

**Provides:**
- Recent messages from the configured channel (text, author, timestamp, thread context)

**Config:**

| Variable | Description |
|---|---|
| `SLACK_TOKEN` | Slack bot token with `channels:history` scope |

**Per-project:** `channel` (channel ID), `max_results`

### PDF

**Component:** `pdf.go`

Extracts text from local PDF files.

**Provides:**
- Text content from PDF files (specifications, design docs, reference material)

**Config:** None (file paths specified per-project).

**Per-project:** `paths` (glob patterns for PDF files relative to project root)

### Web

**Component:** `web.go`

Fetches and extracts text from web pages.

**Provides:**
- Text content from URLs (API documentation, architecture references, external docs)

**Config:** None (URLs specified per-project).

**Per-project:** `urls` (list of URLs to scrape)

## Configuration Summary

### Global Credentials (Environment Variables)

| Variable | Source | Required |
|---|---|---|
| `GITHUB_TOKEN` | GitHub | For GitHub source |
| `JIRA_URL` | Jira | For Jira source |
| `JIRA_TOKEN` | Jira | For Jira source |
| `JIRA_EMAIL` | Jira | For Jira source |
| `LINEAR_TOKEN` | Linear | For Linear source |
| `NOTION_TOKEN` | Notion | For Notion source |
| `SLACK_TOKEN` | Slack | For Slack source |

### Per-Project Configuration

Place a `sources.yaml` in the project root or `.carto/sources.yaml`. The file is optional. Without it, only Git (always available) and sources with global credentials are used.

## Edge Cases

| Scenario | Behavior |
|---|---|
| Source credential is missing | Source is not instantiated. No error, no signals from that source. |
| Source API call fails (network, auth, rate limit) | Error is logged and collected in `Result.Errors`. Other sources continue. Pipeline proceeds. |
| Source returns zero results | Valid outcome. Some projects may have no issues or no relevant Slack messages. |
| External API rate limit hit | The specific source call fails. Other sources are unaffected. The error is non-fatal. |
| `sources.yaml` has invalid syntax | Configuration parsing fails for that file. Sources fall back to global-only configuration. Error logged. |
| PDF file not found | The PDF source logs an error for the missing file and continues with other files in the glob. |
| Web URL is unreachable | The web source logs the error and continues with other URLs. |
| Concurrent source calls exceed local connection limits | Unlikely with typical source counts (< 10), but could occur. Errors would surface as timeouts. |

## Common Questions

**Q1: How do I add a new source type?**
Create a new file in `internal/sources/` implementing the `Source` interface (Name, Configure, FetchSignals). Register it in `BuildRegistry()` in `registry.go` with its credential check. Add per-project configuration parsing in `config.go`.

**Q2: Which sources require credentials?**
All sources except Git and PDF require API credentials. PDF and Web use local files and URLs respectively, configured per-project. Git operates on the local repository.

**Q3: What data does each source provide?**
All sources produce `Signal` structs containing: source name, signal type (issue, pr, doc, message), title, content, metadata (author, date, status, labels), and a reference URL. The specific fields populated vary by source type.

**Q4: How does per-project config interact with global config?**
Global credentials (env vars) determine which sources are available. Per-project config (`sources.yaml`) refines what data each source fetches (e.g., which Jira project, which Slack channel). If a source has global credentials but no per-project config, it uses sensible defaults (e.g., GitHub infers the repo from the git remote).

**Q5: What happens when a source fails during indexing?**
The failure is non-fatal. The error is logged and added to `Result.Errors`. The pipeline continues with whatever signals were collected from other sources. Deep analysis (Phase 4) works with the available data -- fewer signals means less external context, but the index is still functional.

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---|---|---|
| "0 signals collected" when sources are configured | Credentials may be invalid, or per-project config may not match actual data (wrong project key, wrong channel ID) | Verify credentials with a direct API call. Check per-project config values. |
| GitHub source returns no data | Token lacks `repo` scope, or `repo` config does not match the actual repository | Verify token permissions. Check `sources.yaml` for correct `owner/repo`. |
| Jira source fails with 401 | JIRA_TOKEN or JIRA_EMAIL is wrong | Regenerate the API token from Atlassian. Verify the email matches the account. |
| Linear source returns empty results | Team name does not match any Linear team | Check the team slug in Linear settings. |
| Notion source fails | Integration not shared with the target pages/database | In Notion, share the pages with the integration. |
| Slack source fails with "channel_not_found" | Channel ID is wrong, or bot is not in the channel | Verify the channel ID (not the name). Invite the bot to the channel. |
| PDF source finds no files | Glob pattern does not match any files | Check the path pattern relative to the project root. |
