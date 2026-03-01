---
type: config-reference
audience: external
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Configuration Reference

Carto is configured through environment variables. You can set these in your shell, in a `.env` file in the project root, or pass them directly when running Carto.

## Setting Configuration

**Shell export (temporary, current session only):**

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-your-key-here"
export CARTO_FAST_MODEL="claude-haiku-4-5-20251001"
```

**`.env` file (persistent, per project):**

Create a `.env` file in the directory where you run Carto:

```bash
ANTHROPIC_API_KEY=sk-ant-api03-your-key-here
CARTO_FAST_MODEL=claude-haiku-4-5-20251001
MEMORIES_URL=http://localhost:8900
```

**Inline (one-off commands):**

```bash
LLM_PROVIDER=openai LLM_API_KEY=sk-your-key carto index .
```

---

## LLM Configuration

These variables control which LLM provider and models Carto uses for code analysis.

### `LLM_PROVIDER`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | `anthropic` |
| **Required** | No |
| **Options** | `anthropic`, `openai`, `ollama` |

The LLM provider to use for all AI-powered analysis. Carto supports three providers out of the box.

```bash
export LLM_PROVIDER="anthropic"
```

### `LLM_API_KEY`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | Yes (unless using `ANTHROPIC_API_KEY` or Ollama) |

Your API key for the configured LLM provider. This is the general-purpose key variable that works with any provider.

```bash
export LLM_API_KEY="sk-your-api-key"
```

### `ANTHROPIC_API_KEY`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | Yes (if using Anthropic and `LLM_API_KEY` is not set) |

Your Anthropic API key. If both `LLM_API_KEY` and `ANTHROPIC_API_KEY` are set, `LLM_API_KEY` takes precedence.

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-your-key-here"
```

> **Note on precedence:** When `LLM_PROVIDER` is set to `anthropic`, Carto checks for `LLM_API_KEY` first, then falls back to `ANTHROPIC_API_KEY`. This lets you use the Anthropic-specific variable without also setting the generic one.

### `LLM_BASE_URL`

| Detail | Value |
|--------|-------|
| **Type** | string (URL) |
| **Default** | Provider-specific |
| **Required** | No (required for Ollama or custom endpoints) |

The base URL for the LLM API. You only need to set this if you're using a non-default endpoint, such as a local Ollama server or an API proxy.

```bash
# For Ollama running locally:
export LLM_BASE_URL="http://localhost:11434"

# For an OpenAI-compatible proxy:
export LLM_BASE_URL="https://your-proxy.example.com/v1"
```

### `CARTO_FAST_MODEL`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | `claude-haiku-4-5-20251001` |
| **Required** | No |

The model used for high-volume, fast operations like extracting atom summaries from individual code chunks. This model is called many times during indexing, so a fast, cost-effective model works best.

```bash
export CARTO_FAST_MODEL="claude-haiku-4-5-20251001"
```

### `CARTO_DEEP_MODEL`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | `claude-opus-4-6` |
| **Required** | No |

The model used for expensive, high-quality analysis like cross-component wiring, zone identification, and architectural blueprint generation. This model is called fewer times but handles more complex reasoning.

```bash
export CARTO_DEEP_MODEL="claude-opus-4-6"
```

### `CARTO_MAX_CONCURRENT`

| Detail | Value |
|--------|-------|
| **Type** | integer |
| **Default** | `10` |
| **Required** | No |

The maximum number of concurrent LLM requests Carto makes during indexing. Lower this if you're hitting rate limits; raise it if you have a high-throughput API plan.

```bash
export CARTO_MAX_CONCURRENT=5
```

---

## Storage Configuration

These variables control how Carto connects to the Memories server where index data is stored and retrieved.

### `MEMORIES_URL`

| Detail | Value |
|--------|-------|
| **Type** | string (URL) |
| **Default** | `http://localhost:8900` |
| **Required** | No |

The URL of your Memories server. Carto stores all index layers here and queries it when you run `carto query`.

```bash
export MEMORIES_URL="http://localhost:8900"
```

### `MEMORIES_API_KEY`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | No (depends on your Memories server configuration) |

The API key for authenticating with the Memories server. Only required if your Memories server has authentication enabled.

```bash
export MEMORIES_API_KEY="your-memories-api-key"
```

---

## Source Credentials

These variables provide authentication tokens for external source integrations. Each one is optional -- you only need to set the ones for services you want Carto to pull context from.

### `GITHUB_TOKEN`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | No (required for GitHub source integration) |

A GitHub personal access token. Carto uses this to fetch issues, pull requests, and repository metadata for richer index context.

```bash
export GITHUB_TOKEN="ghp_your-github-token"
```

### `JIRA_URL`

| Detail | Value |
|--------|-------|
| **Type** | string (URL) |
| **Default** | -- |
| **Required** | No (required for Jira source integration) |

The base URL of your Jira instance.

```bash
export JIRA_URL="https://yourcompany.atlassian.net"
```

### `JIRA_TOKEN`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | No (required for Jira source integration) |

Your Jira API token. Used together with `JIRA_URL` to fetch issues and project data.

```bash
export JIRA_TOKEN="your-jira-api-token"
```

### `LINEAR_TOKEN`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | No (required for Linear source integration) |

A Linear API key. Carto uses this to fetch issues and project data from Linear.

```bash
export LINEAR_TOKEN="lin_api_your-linear-token"
```

### `NOTION_TOKEN`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | No (required for Notion source integration) |

A Notion integration token. Carto uses this to fetch pages and databases from Notion for additional project context.

```bash
export NOTION_TOKEN="ntn_your-notion-token"
```

### `SLACK_TOKEN`

| Detail | Value |
|--------|-------|
| **Type** | string |
| **Default** | -- |
| **Required** | No (required for Slack source integration) |

A Slack bot token. Carto uses this to pull relevant channel messages and threads that provide context about your codebase.

```bash
export SLACK_TOKEN="xoxb-your-slack-bot-token"
```

---

## Docker Configuration

These variables are relevant when running Carto via Docker Compose.

### `PROJECTS_DIR`

| Detail | Value |
|--------|-------|
| **Type** | string (directory path) |
| **Default** | `~/projects` |
| **Required** | No |

The host directory that gets mounted into the Docker container. Carto scans and indexes projects from this directory.

```bash
export PROJECTS_DIR="/home/user/my-projects"
```

When using Docker Compose, this directory is mounted as a volume so that Carto inside the container can access your project files.

---

## Example `.env` File

Here's a complete example showing a typical configuration:

```bash
# LLM (using Anthropic)
ANTHROPIC_API_KEY=sk-ant-api03-your-key-here
CARTO_FAST_MODEL=claude-haiku-4-5-20251001
CARTO_DEEP_MODEL=claude-opus-4-6
CARTO_MAX_CONCURRENT=10

# Storage
MEMORIES_URL=http://localhost:8900
MEMORIES_API_KEY=your-memories-key

# Sources (enable what you need)
GITHUB_TOKEN=ghp_your-github-token
JIRA_URL=https://yourcompany.atlassian.net
JIRA_TOKEN=your-jira-token
# LINEAR_TOKEN=
# NOTION_TOKEN=
# SLACK_TOKEN=

# Docker
PROJECTS_DIR=~/projects
```

---

## Provider-Specific Setup

### Anthropic (Default)

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-your-key-here"
# No other config needed -- Anthropic is the default provider
```

### OpenAI

```bash
export LLM_PROVIDER="openai"
export LLM_API_KEY="sk-your-openai-key"
export CARTO_FAST_MODEL="gpt-4o-mini"
export CARTO_DEEP_MODEL="gpt-4o"
```

### Ollama (Local)

```bash
export LLM_PROVIDER="ollama"
export LLM_BASE_URL="http://localhost:11434"
export CARTO_FAST_MODEL="llama3.2"
export CARTO_DEEP_MODEL="llama3.2:70b"
# No API key needed for local Ollama
```
