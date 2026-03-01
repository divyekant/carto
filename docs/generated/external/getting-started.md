---
type: getting-started
audience: external
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Getting Started with Carto

Carto is a codebase intelligence tool that helps AI assistants understand your code. It scans your project, builds a rich semantic index, and generates skill files that tools like Claude and Cursor can use to give you better, context-aware answers.

This guide walks you through installing Carto, indexing your first project, and running your first query.

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
mv carto /usr/local/bin/
```

### Step 2: Configure Your API Key

Set your LLM API key as an environment variable. If you're using Anthropic (the default provider):

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

### Step 4: Index Your Project

Point Carto at a codebase to scan and index it:

```bash
carto index /path/to/your/project
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

### Step 5: Query Your Project

Now you can ask questions about your codebase in natural language:

```bash
carto query "How does authentication work?"
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
carto query "How does authentication work?" --tier mini

# Balanced detail (~50KB of context, default)
carto query "How does authentication work?" --tier standard

# Deep dive (~500KB of context)
carto query "How does authentication work?" --tier full
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

## Next Steps

Now that you have Carto running, here's where to go next:

- **Generate skill files** for your AI assistant: `carto patterns /path/to/project`
- **Explore the Web UI** for visual project management: `carto serve --port 8950`
- **Connect external sources** like GitHub issues or Jira tickets -- see the [Configuration Reference](config-reference.md)
- **Use the REST API** to integrate Carto into your tooling -- see the [API Reference](api-reference.md)
- **Troubleshoot issues** with the [Error Reference](error-reference.md)
- **Review what's new** in the [Changelog](changelog.md)
