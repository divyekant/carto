---
id: uc-005
type: use-case
audience: internal
topic: Generating AI Assistant Context Files
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Use Case: Generating AI Assistant Context Files

## Trigger

Skill file generation is triggered in one of two ways:

1. **Pipeline Phase 6:** Automatically after a successful indexing run (`carto index /path/to/project`), unless `SkipSkillFiles` is set.
2. **Standalone command:** `carto patterns /path/to/project [--format claude|cursor|all]`

## Preconditions

1. The project has been indexed at least once (synthesis data exists in Memories).
2. Memories server is running and reachable (for the standalone command or during pipeline Phase 6).
3. The project directory is writable (skill files are written to the project root).

## Primary Flow: Pipeline Phase 6

### Step 1: Gather Synthesis Data

After Phase 5 (Store) completes, the pipeline passes the in-memory SystemSynthesis and ModuleAnalyses directly to the skill file generator. No Memories query is needed because the data is already available from Phase 4.

The synthesis data includes:
- Blueprint: system-wide architecture and design rationale
- Module analyses: per-module purpose, wiring, zones
- Patterns: cross-cutting coding conventions

### Step 2: Generate Content

The generator transforms synthesis data into structured text:

1. Renders the project header with the project name.
2. Writes the architecture summary from the blueprint.
3. Lists each module with its description and key components.
4. Enumerates business domains / zones.
5. Lists discovered coding patterns.
6. Appends Memories query instructions (curl commands with the project's source prefix).
7. Appends Memories write-back instructions for the active index workflow.

### Step 3: Detect Existing Files

For each target file (CLAUDE.md, .cursorrules, or both depending on `--format`):

1. Check if the file exists in the project root.
2. If it exists, read its current content.
3. Search for the `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->` markers.

### Step 4: Inject or Replace Content

**File does not exist:**
- Create the file.
- Write the markers with generated content.

**File exists with markers:**
- Preserve all content before `<!-- BEGIN CARTO INDEX -->`.
- Replace everything between the markers with new generated content.
- Preserve all content after `<!-- END CARTO INDEX -->`.

**File exists without markers:**
- Append a newline, the BEGIN marker, generated content, and the END marker to the end of the file.
- All existing content is preserved.

### Step 5: Write Files

Write the final content to disk. Report success or failure in the pipeline result.

## Variation: Standalone Generation

### Trigger

```bash
carto patterns /path/to/project --format claude
```

### Flow Differences

- **Step 1 (Gather):** Instead of using in-memory data from the pipeline, the generator queries Memories for the project's synthesis data:
  - Fetches blueprint: `source=carto/{project}/layer:blueprint`
  - Fetches patterns: `source=carto/{project}/layer:patterns`
  - Fetches module analyses: `source=carto/{project}/{module}/layer:zones` for each module
- Steps 2-5 are identical to the pipeline flow.

### Use Cases for Standalone

- Regenerating skill files after changing the `--format` preference.
- Updating skill files after manually editing Memories data.
- Generating skill files for a project indexed on a different machine (if both share the same Memories server).

## Variation: Format-Specific Generation

### Claude Only

```bash
carto patterns /path/to/project --format claude
```

Generates only `CLAUDE.md`. Skips `.cursorrules`.

### Cursor Only

```bash
carto patterns /path/to/project --format cursor
```

Generates only `.cursorrules`. Skips `CLAUDE.md`.

### Both (Default)

```bash
carto patterns /path/to/project --format all
```

Generates both `CLAUDE.md` and `.cursorrules`.

## Edge Cases

| Scenario | Behavior |
|---|---|
| No synthesis data available | No skill files are generated. A warning is logged. For pipeline mode, this means Phase 4 failed. For standalone mode, it means no data exists in Memories for the project. |
| File is read-only | Write fails with a permission error. Logged as a non-fatal error. |
| File has markers but content between them is manually modified | The manual modifications are replaced with freshly generated content. Content outside markers is preserved. |
| Multiple CLAUDE.md files in subdirectories | Carto writes only to the project root. Subdirectory files are not affected. |
| `.cursorrules` contains non-Carto rules | Non-Carto rules placed outside the markers are preserved. Rules between the markers are replaced. |
| Project name contains special characters | The project name is used as-is in the generated content and Memories query commands. Special characters may need URL encoding in the curl commands. |
| Concurrent generation (two Carto instances) | Both instances read and write the same file. Last writer wins. Content may be inconsistent. Avoid concurrent generation for the same project. |

## Data Impact

**Written:**

| Location | Content |
|---|---|
| Disk: `{project_root}/CLAUDE.md` | Skill file for Claude-based tools (if format is `claude` or `all`) |
| Disk: `{project_root}/.cursorrules` | Skill file for Cursor IDE (if format is `cursor` or `all`) |

**Read:**

| Location | Content |
|---|---|
| Memories: `carto/{project}/layer:blueprint` | System architecture and design rationale |
| Memories: `carto/{project}/layer:patterns` | Cross-cutting coding patterns |
| Memories: `carto/{project}/{module}/layer:zones` | Business domains per module |

**Not Modified:**

- Memories entries are only read, not modified, during skill file generation.
- Existing file content outside the Carto markers is preserved.

## Post-Conditions

1. Skill files exist in the project root with up-to-date content between the Carto markers.
2. User-authored content outside the markers is intact.
3. The generated content includes active index instructions (query and write-back curl commands).
4. AI assistants opening the project will see the indexed context and can use the active index workflow.
