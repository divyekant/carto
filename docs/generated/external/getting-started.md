---
type: getting-started
audience: external
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Getting Started with Carto

Carto is a codebase intelligence tool that helps AI assistants understand your code. It scans your project, builds a rich semantic index, and generates skill files that tools like Claude and Cursor can use to give you better, context-aware answers.

This guide walks you through installing Carto, configuring it, indexing your first project, and running your first query.

## Prerequisites

Before you begin, make sure you have the following:

- **Go 1.25 or later** with CGO enabled (required for AST parsing)
- **An LLM API key** from Anthropic, OpenAI, or a running Ollama instance
- **A Memories server** running at `http://localhost:8900` (Carto uses this to store and retrieve index data)

## Quick Start

### Step 1: Build Carto

Clone the repository and build the binary:

```bash
git clone https://github.com/anthropics/carto.git
cd carto
go build -o carto ./cmd/carto
```

You should now have a `carto` binary in the current directory. You can move it to a directory on your `PATH` for convenience:

```bash
mv carto ~/bin/
```

### Step 2: Run the Setup Wizard

The `carto init` command walks you through configuring your LLM provider, API key, and Memories server:

```bash
carto init
```

The wizard prompts you for each setting. Press Enter to accept the default values shown in brackets.

If you prefer to configure non-interactively (for example, in a script or Dockerfile):

```bash
carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "$ANTHROPIC_API_KEY" \
  --memories-url http://localhost:8900
```

You can also set your API key directly as an environment variable instead of running `init`:

```bash
export ANTHROPIC_API_KEY="sk-ant-api03-your-key-here"
```

For other providers, see the [Configuration Reference](config-reference.md).

### Step 3: Start the Memories Server

Carto stores its index in a Memories server. Make sure it's running on the default port:

```bash
# If using Docker:
docker run -p 8900:8900 memories-server

# Verify it's reachable:
curl http://localhost:8900/health
```

You should see a healthy response confirming the server is ready.

### Step 4: Verify Your Setup

Run the doctor command to check that everything is configured correctly:

```bash
carto doctor
```

You should see green checkmarks next to each requirement. If anything is missing, the doctor will tell you exactly what to fix.

### Step 5: Index Your Project

Point Carto at a codebase to scan and index it:

```bash
carto index --path /path/to/your/project --name my-project
```

Carto will scan your files, parse the code into semantic chunks, analyze relationships using your LLM, and store everything in Memories. You'll see progress output like this:

```
Scanning /path/to/your/project...
Found 142 files across 3 modules
Chunking and extracting atoms... [====================] 142/142
Analyzing history and signals... done
Running deep analysis... done
Storing index layers... done
Generating skill files... done

Indexed 142 files in 47s
```

### Step 6: Query Your Project

Now you can ask questions about your codebase in natural language:

```bash
carto query --project my-project "How does authentication work?"
```

Carto retrieves the most relevant context from your index and returns a focused answer:

```
Authentication is handled by the auth middleware in pkg/auth/middleware.go.
Incoming requests are validated using JWT tokens issued by the /login
endpoint. Token verification uses the RS256 algorithm with keys loaded
from the AUTH_PUBLIC_KEY environment variable...
```

You can control how much detail you get with the `--tier` flag:

```bash
# Quick summary (~5KB of context)
carto query --project my-project "How does authentication work?" --tier mini

# Balanced detail (~50KB of context, default)
carto query --project my-project "How does authentication work?" --tier standard

# Deep dive (~500KB of context)
carto query --project my-project "How does authentication work?" --tier full
```

## What Happens During Indexing?

When you run `carto index`, Carto builds a 7-layer understanding of your codebase:

1. **Map** -- discovers files and modules
2. **Atoms** -- parses code into meaningful chunks with summaries
3. **History** -- extracts git history for change patterns
4. **Signals** -- pulls in external context (issues, docs, Slack threads)
5. **Wiring** -- maps how components connect and depend on each other
6. **Zones** -- identifies logical areas of responsibility
7. **Blueprint** -- creates a high-level architectural overview

All of this is stored in Memories so that future queries are fast and accurate.

## Available Commands

Carto includes 18 commands covering setup, indexing, querying, project management, and operations:

| Command | What It Does |
|---------|-------------|
| `init` | Run the configuration wizard |
| `index` | Scan and index a codebase |
| `query` | Search the index with natural language |
| `modules` | List discovered modules |
| `patterns` | Generate skill files (CLAUDE.md, .cursorrules) |
| `status` | Check indexing state |
| `serve` | Start the web UI and REST API |
| `projects` | Manage projects |
| `sources` | Manage external signal sources |
| `config` | View/update configuration |
| `auth` | Manage API keys and credentials |
| `doctor` | Run environment health checks |
| `export` | Export index data as NDJSON |
| `import` | Import NDJSON index data |
| `logs` | Query/tail the audit log |
| `completions` | Generate shell completion scripts |
| `upgrade` | Check for new versions |
| `version` | Show version info |
| `about` | Display product information |

For full details on every command and flag, see the [CLI Reference](features/feat-006-cli.md).

## Next Steps

Now that you have Carto running, here's where to go next:

- **Generate skill files** for your AI assistant: `carto patterns --project my-project`
- **Explore the Web UI** for visual project management: `carto serve --port 8950`
- **Set up shell completions** for faster CLI usage: `carto completions bash` (or zsh/fish)
- **Connect external sources** like GitHub issues or Jira tickets -- see the [Configuration Reference](config-reference.md)
- **Use the REST API** to integrate Carto into your tooling -- see the [API Reference](api-reference.md)
- **Learn CI/CD integration** with JSON output -- see the [CLI Reference](features/feat-006-cli.md)
- **Back up your index** with export/import -- see the [Cookbook](cookbook.md)
- **Troubleshoot issues** with the [Error Reference](error-reference.md)
- **Review what's new** in the [Changelog](changelog.md)
