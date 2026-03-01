---
id: feat-005
type: feature-doc
audience: external
topic: Skill File Generation
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Skill File Generation

Carto generates skill files that give AI coding assistants instant understanding of your codebase. Instead of an AI assistant starting from scratch every session, it reads the skill file and immediately knows your project's architecture, patterns, modules, and conventions.

Carto produces two formats:

- **CLAUDE.md** -- for Claude Code and other tools that read CLAUDE.md
- **.cursorrules** -- for Cursor and compatible editors

These files are written to your project root and are designed to be committed to your repository so every developer (and every AI session) benefits from them.

## How to Use It

```bash
carto patterns .
```

This generates both `CLAUDE.md` and `.cursorrules` in your project directory.

Skill files are also generated automatically at the end of every `carto index` run, so you often don't need to run this command separately.

## Configuration

| Flag | Description | Default |
|------|-------------|---------|
| `--format claude` | Generate only CLAUDE.md | -- |
| `--format cursor` | Generate only .cursorrules | -- |
| `--format all` | Generate both formats | `all` |

## Examples

**Generate both skill files (default):**

```bash
carto patterns .
```

Writes `CLAUDE.md` and `.cursorrules` to the current directory.

**Generate only CLAUDE.md:**

```bash
carto patterns . --format claude
```

**Generate only .cursorrules:**

```bash
carto patterns . --format cursor
```

**Generate for a specific project path:**

```bash
carto patterns /path/to/my-project
```

## What's Inside a Skill File

A generated skill file includes:

- **Project overview:** What the project does and its core purpose
- **Architecture summary:** How the system is structured, major components, and their relationships
- **Module map:** Each module with its purpose, key exports, and dependencies
- **Code patterns:** Design patterns, conventions, and idioms used in the codebase
- **Keeping the index current:** Instructions for updating the Carto index when code changes

The content is wrapped in Carto markers (`<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->`), so Carto knows which sections it owns.

## Preserving Your Own Content

If you already have a `CLAUDE.md` or `.cursorrules` with your own instructions, Carto won't overwrite them. It only updates the content between its own markers:

```markdown
# My Project

These are my custom instructions that Carto will never touch.

<!-- BEGIN CARTO INDEX -->
... Carto-generated content goes here ...
<!-- END CARTO INDEX -->

More of my own notes below.
```

You can freely add content above and below the Carto markers. On the next `carto patterns` or `carto index` run, only the section between the markers is updated.

## When to Regenerate

Skill files are regenerated automatically during `carto index`, so they stay current as your codebase evolves. You might want to run `carto patterns` explicitly when:

- You've updated the index and want to refresh skill files without a full re-index
- You want to generate a specific format you didn't generate before
- You've deleted a skill file and want to recreate it

## Limitations

- **Requires a successful index with deep analysis.** Skill files draw from all seven layers of the Carto index. If deep analysis didn't complete (for example, due to LLM errors), the skill file may be incomplete.
- **Existing content is preserved.** Carto only modifies content between its `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->` markers. Everything outside those markers is left untouched.

## Related

- [Indexing Pipeline](feat-001-indexing-pipeline.md) -- skill files are generated as the final indexing phase
