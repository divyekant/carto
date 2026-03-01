---
id: fh-005
type: feature-handoff
audience: internal
topic: Skill File Generation
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Handoff: Skill File Generation

## What It Does

The `patterns/` package generates skill files -- structured context documents that AI coding assistants use to understand a codebase. Carto produces two formats: `CLAUDE.md` (for Claude-based tools like Claude Code) and `.cursorrules` (for Cursor IDE). These files contain the indexed knowledge about the project's architecture, modules, patterns, and conventions, enabling AI assistants to provide more accurate, context-aware assistance.

Skill file generation is Phase 6 of the indexing pipeline. It transforms the SystemSynthesis and ModuleAnalyses from Phase 4 into human- and machine-readable context files.

## How It Works

### Content Structure

A generated skill file contains the following sections:

1. **Project name and overview:** A one-line description of what the project is.
2. **Architecture summary:** High-level description of the system's structure, purpose, and design rationale (from the blueprint layer).
3. **Module descriptions:** Per-module summaries including purpose, key components, and exports.
4. **Business domains / Zones:** Functional areas identified during deep analysis (e.g., "Authentication", "Data Pipeline", "API Layer").
5. **Coding patterns:** Cross-cutting conventions discovered during synthesis (e.g., "Error wrapping with context", "Repository pattern for data access").
6. **Memory query instructions:** Curl commands showing how to query the Memories API for more context about the project.
7. **Memory write-back instructions:** Curl commands showing how to update the index after code changes.

### Marker-Based Injection

Skill files use markers to delineate Carto-managed content:

```
<!-- BEGIN CARTO INDEX -->
... generated content ...
<!-- END CARTO INDEX -->
```

**Preservation behavior:**

- If the file does not exist, Carto creates it with the markers and generated content.
- If the file exists and contains the markers, Carto replaces only the content between the markers. Everything before `<!-- BEGIN CARTO INDEX -->` and after `<!-- END CARTO INDEX -->` is preserved intact.
- If the file exists but does not contain the markers, Carto appends the markers and content at the end of the file.

This ensures that user-authored instructions, project-specific rules, and custom configurations outside the markers are never overwritten.

### Active Index Workflow

The generated skill files include instructions for AI assistants to participate in an active index workflow:

1. **Query before edit:** Before making changes, the assistant queries Memories for relevant context about the affected modules and patterns.
2. **Write back after changes:** After completing a significant change (new feature, bug fix, refactor), the assistant writes a summary back to Memories to keep the index current.

This creates a feedback loop where the index improves over time as the codebase evolves.

### Format Selection

The `--format` flag (or `SkillFileFormat` config) controls which file(s) are generated:

| Format | File Generated | Target Tool |
|---|---|---|
| `claude` | `CLAUDE.md` | Claude Code, Claude CLI |
| `cursor` | `.cursorrules` | Cursor IDE |
| `all` | Both files | Both tools |

The content is identical between formats. Only the file name and minor formatting conventions differ.

## User-Facing Behavior

### As Part of Pipeline

When the pipeline completes Phase 5 (Store), Phase 6 automatically generates skill files unless `SkipSkillFiles` is set. The files are written to the project root directory.

### Standalone Command

```bash
carto patterns /path/to/project [--format claude|cursor|all]
```

This command generates skill files from the currently stored data in Memories without re-running the full indexing pipeline. Useful for regenerating skill files after manual Memories edits or to change the format.

## Configuration

| Setting | Default | Description |
|---|---|---|
| `--format` | `all` | Skill file format: `claude`, `cursor`, `all` |
| `SkipSkillFiles` | `false` | Skip Phase 6 entirely (pipeline config) |
| `--project` | Directory name | Project name used to query Memories for synthesis data |

No environment variables are specific to skill file generation. The feature relies on Memories configuration (`MEMORIES_URL`, `MEMORIES_API_KEY`) to fetch the synthesis data.

## Edge Cases

| Scenario | Behavior |
|---|---|
| No SystemSynthesis available (Phase 4 failed completely) | No skill files are generated. A warning is logged. |
| Existing file has content but no markers | Markers and generated content are appended to the end of the file. Existing content is preserved. |
| Existing file has markers with content between them | Content between markers is replaced. Content outside markers is preserved. |
| Existing file has malformed markers (e.g., only BEGIN, no END) | The system treats the file as having no markers. Content is appended at the end. |
| File is read-only | Write fails. Error is logged. Non-fatal. |
| Project has zero modules | Skill file is generated with project-level content only (blueprint, patterns). No module section. |
| Very large project (many modules) | Skill file may be large. All modules are included. No truncation. |
| `--format cursor` but `.cursorrules` already has user rules | User rules outside the markers are preserved. |

## Common Questions

**Q1: Will generating skill files overwrite my custom CLAUDE.md instructions?**
No. Carto uses marker-based injection. Any content you write outside the `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->` markers is preserved. Only the content between the markers is replaced on each generation.

**Q2: Can I manually edit the generated content between the markers?**
You can, but the edits will be overwritten the next time skill files are generated (during re-indexing or `carto patterns`). For persistent customizations, place your content outside the markers.

**Q3: What if I delete the markers from the file?**
The next generation will treat the file as having no markers and append a new marker block at the end. Your existing content (including what was previously between the markers) remains.

**Q4: Do skill files work without a running Memories server?**
Skill files are static once generated -- they do not require Memories to be read by AI assistants. However, the active index instructions within the skill file (query and write-back commands) do require Memories to be running when the AI assistant executes them.

**Q5: How do I regenerate skill files without re-indexing?**
Run `carto patterns /path/to/project`. This fetches the current synthesis data from Memories and regenerates the skill files without running the full pipeline.

## Troubleshooting

| Symptom | Likely Cause | Resolution |
|---|---|---|
| No skill files generated after indexing | Phase 4 (Deep Analysis) failed, producing no synthesis data. Or `SkipSkillFiles` is set. | Check indexing output for Phase 4 errors. Verify `SkipSkillFiles` is not set. |
| Markers are duplicated in the file | Multiple runs appended markers because existing markers were not detected (e.g., whitespace differences) | Manually remove duplicate marker blocks. Ensure the markers are exactly `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->` on their own lines. |
| User content outside markers was overwritten | This should not happen. Possible bug if it occurs. | Restore the file from version control. File a bug report with the before/after file content. |
| Skill file content is empty between markers | Synthesis data in Memories is empty or corrupted | Re-index the project with `--full`. Check Memories for synthesis data. |
| `carto patterns` fails with "no data found" | Project name does not match stored data, or Memories is unreachable | Verify the project name. Check Memories connectivity. |
| Skill file is very large (> 100 KB) | Project has many modules producing verbose synthesis | This is expected for large projects. The content can be reviewed and manually trimmed if needed. |
