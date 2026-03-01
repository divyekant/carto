---
id: feat-001
type: feature-doc
audience: external
topic: Indexing Pipeline
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Indexing Pipeline

Carto's indexing pipeline scans your codebase, breaks it into meaningful chunks, and builds a layered semantic understanding using LLMs. The result is a rich, searchable index that AI coding assistants can use to give you better answers about your code.

When you run an index, Carto walks through six phases: scanning files, extracting code atoms (functions, types, exports), analyzing git history and external signals, performing deep cross-component analysis, storing everything in Memories, and generating skill files. You get progress output at each step so you always know what's happening.

## How to Use It

### 1. Set your LLM API key

```bash
export LLM_API_KEY="your-anthropic-api-key"
```

### 2. Run the indexer

```bash
carto index .
```

### 3. Verify the index

```bash
carto status
```

You should see your project listed with file counts, atom counts, and the timestamp of the last successful index.

## Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `--incremental` | Only re-index files that changed since the last run | `true` |
| `--full` | Force a complete re-index of all files | `false` |
| `--module <name>` | Index only a specific module within the project | all modules |
| `--project <name>` | Set the project name for this index | directory name |
| `--all` | Index all configured projects | `false` |
| `--changed` | Index only projects with detected changes | `false` |

## Examples

**Index the current directory:**

```bash
carto index .
```

This is the most common usage. Carto detects your project name from the directory, scans all supported files, and builds the full index.

**Index a specific project path:**

```bash
carto index /path/to/my-project --project my-app
```

**Incremental re-index (default behavior):**

```bash
carto index .
```

On subsequent runs, Carto automatically detects which files changed (using SHA-256 hashes) and only re-processes those files. This makes re-indexing fast -- typically seconds instead of minutes.

**Force a full re-index:**

```bash
carto index . --full
```

Use this if you want to rebuild the entire index from scratch, for example after upgrading Carto or changing LLM providers.

**Index a single module:**

```bash
carto index . --module auth
```

If your project has multiple modules (detected automatically), you can target just one. This is useful when you've made changes to a specific area and want a quick update.

## What You'll See

When you run `carto index`, you'll see progress output like this:

```
Scanning files...
  Found 142 files across 3 modules (auth, api, core)

Extracting atoms...
  Processed 142 files, extracted 847 atoms

Analyzing history & signals...
  Git history: 1,204 commits analyzed
  External signals: 12 GitHub PRs, 8 Jira tickets

Deep analysis...
  Wiring: 23 cross-module connections mapped
  Zones: 5 architectural zones identified
  Blueprint: project architecture summarized

Storing index...
  Stored 847 atoms + 7 analysis layers

Generating skill files...
  Wrote CLAUDE.md (12.4 KB)
  Wrote .cursorrules (11.8 KB)

Done in 2m 34s
```

## Limitations

- **CGO required:** Carto uses Tree-sitter for AST parsing, which requires CGO (a C compiler). Make sure `gcc` is available on your system.
- **AST parsing for 6 languages:** Full AST-based chunking is available for Go, TypeScript, JavaScript, Python, Java, and Rust. Carto detects 30+ languages but uses line-based chunking for languages without AST support.
- **Large codebases take time:** A first-time index of a large codebase (thousands of files) may take several minutes depending on your LLM provider's throughput. Incremental re-indexes are much faster.
- **Memories server required:** The index is stored in a Memories server, which must be running at `http://localhost:8900` (or your configured URL).

## Related

- [Querying & Retrieval](feat-004-storage-retrieval.md) -- search your indexed codebase
- [Skill File Generation](feat-005-skill-file-generation.md) -- generate AI assistant context files
- [External Sources](feat-003-unified-sources.md) -- enrich your index with tickets, PRs, and docs
