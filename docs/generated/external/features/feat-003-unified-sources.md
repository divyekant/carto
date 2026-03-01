---
id: feat-003
type: feature-doc
audience: external
topic: External Sources
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# External Sources

Your codebase doesn't exist in isolation. Tickets describe why code was written. Pull requests capture review discussions. Documentation explains design decisions. Carto's external sources feature lets you pull context from these tools into your index, giving AI assistants a richer understanding of your project.

When you connect external sources, Carto fetches relevant context during the indexing pipeline's signals phase and weaves it into the semantic index alongside your code.

## Supported Sources

| Source | What It Brings | Credential Required |
|--------|---------------|---------------------|
| **Git** | Commit history, file change frequency, authorship | None (auto-detected) |
| **GitHub** | Pull requests, issues, reviews, discussions | `GITHUB_TOKEN` |
| **Jira** | Tickets, epics, sprint context | `JIRA_URL` + `JIRA_TOKEN` |
| **Linear** | Issues, projects, cycles | `LINEAR_TOKEN` |
| **Notion** | Pages, databases, documentation | `NOTION_TOKEN` |
| **Slack** | Channel messages, threads | `SLACK_TOKEN` |
| **PDF** | Document content extraction | None |
| **Web** | Web page content | None |

## How to Use It

### 1. Set your source credentials

Export the tokens for the sources you want to connect:

```bash
export GITHUB_TOKEN="ghp_..."
export JIRA_URL="https://your-org.atlassian.net"
export JIRA_TOKEN="your-jira-api-token"
```

### 2. Configure sources for your project

You can configure which sources apply to each project through the CLI:

```bash
carto sources add github --project my-app --repo "org/repo"
carto sources add jira --project my-app --board "ENG"
```

Or configure them through the web dashboard at `http://localhost:8950` in the project detail view.

### 3. Index your project

```bash
carto index .
```

During the signals phase, Carto fetches context from all configured sources and integrates it into the index.

## Examples

**Add GitHub context to your project:**

```bash
export GITHUB_TOKEN="ghp_..."
carto sources add github --project my-app --repo "my-org/my-app"
carto index .
```

Carto pulls in recent PRs, issues, and review discussions related to your codebase.

**Add Jira tickets:**

```bash
export JIRA_URL="https://myteam.atlassian.net"
export JIRA_TOKEN="your-token"
carto sources add jira --project my-app --board "BACKEND"
carto index .
```

**Configure sources through the web UI:**

```bash
carto serve
```

Open `http://localhost:8950`, navigate to your project, and use the Sources section to add and configure external sources with a visual interface.

**Add a PDF or web source:**

```bash
carto sources add pdf --project my-app --path "/path/to/design-doc.pdf"
carto sources add web --project my-app --url "https://docs.example.com/api"
```

These are useful for pulling in architecture documents, API specifications, or other reference material that isn't in your repository.

## How Sources Enrich Your Index

When you query your index after adding sources, the results include context from those external tools. For example:

- A query about authentication might surface the related Jira epic alongside the code
- A query about a recent refactor might include the PR discussion where the approach was decided
- A query about an API endpoint might reference the Notion doc that describes its design

This gives AI assistants the "why" behind the code, not just the "what."

## Limitations

- **Each source needs its own credentials.** You'll need API tokens with read access for each service you want to connect.
- **Source failures don't block indexing.** If a source is misconfigured or unreachable, Carto logs a warning and continues indexing your code. Your index will still be built -- it just won't include that source's context.
- **Token permissions:** Make sure your tokens have sufficient read permissions. For GitHub, you need `repo` scope. For Jira, read access to the relevant boards. For Slack, access to the channels you want to index.

## Related

- [Indexing Pipeline](feat-001-indexing-pipeline.md) -- how sources fit into the indexing phases
- [Querying & Retrieval](feat-004-storage-retrieval.md) -- search across code and source context
- [LLM Providers](feat-002-llm-integration.md) -- configure the LLM that processes source content
