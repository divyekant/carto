---
id: ts-005
type: troubleshooting
audience: internal
topic: Skill File Generation Issues
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Troubleshooting: Skill File Generation Issues

## Quick Check

1. **Indexing completed successfully:** Check the pipeline output for Phase 4 (Deep Analysis) and Phase 6 (Skill Files) status.
2. **Synthesis data exists in Memories:** `curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count?source=carto/{project}/layer:blueprint"` should return a non-zero count.
3. **Project directory is writable:** `touch /path/to/project/.write-test && rm /path/to/project/.write-test`

---

## Symptom: "No skill files generated"

After indexing or running `carto patterns`, no CLAUDE.md or .cursorrules file appears in the project root.

### Diagnostic Steps

1. Check if `SkipSkillFiles` is set in the pipeline configuration:
   - This flag explicitly disables Phase 6.
   - It may be set via CLI flag or configuration.

2. Check if Phase 4 (Deep Analysis) succeeded:
   - If the deep-tier LLM was unavailable or all analysis calls failed, no SystemSynthesis is produced.
   - No synthesis = no skill files.

3. Check if synthesis data exists in Memories:
   ```bash
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count?source=carto/{project}/layer:blueprint"
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories/count?source=carto/{project}/layer:patterns"
   ```

4. Check the pipeline output or logs for Phase 6 errors (file write failures, permission issues).

5. Verify the `--format` flag was not set to an unexpected value.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| `SkipSkillFiles` is enabled | Remove the flag or set it to `false`. |
| Phase 4 failed (no synthesis data) | Fix the Phase 4 issue first (see ts-001). Re-index the project. |
| Synthesis data is missing from Memories | Re-index with `--full` to regenerate and store synthesis data. |
| File write failed (permissions) | Ensure the project directory is writable by the user running Carto. |
| `--format` set to a format that was not checked | Verify the file name: `CLAUDE.md` for `claude` format, `.cursorrules` for `cursor` format. Check both if `--format all`. |

---

## Symptom: Markers Corrupted or Duplicated

The skill file contains multiple `<!-- BEGIN CARTO INDEX -->` markers, mismatched markers, or garbled marker text.

### Diagnostic Steps

1. Open the file and search for all occurrences of "CARTO INDEX":
   ```bash
   grep -n "CARTO INDEX" /path/to/project/CLAUDE.md
   ```

2. Count the markers:
   - Expected: exactly one `<!-- BEGIN CARTO INDEX -->` and one `<!-- END CARTO INDEX -->`.
   - If there are more, duplication has occurred.

3. Check if the markers have extra whitespace, different casing, or other variations:
   - The markers must be exactly `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->` (case-sensitive, no extra characters).

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Multiple indexing runs appended new markers because existing ones were not detected | Manually edit the file: remove duplicate marker blocks, keeping only one BEGIN/END pair with the latest content. |
| Markers were manually edited (extra spaces, different case) | Restore the exact marker text: `<!-- BEGIN CARTO INDEX -->` and `<!-- END CARTO INDEX -->`. |
| File was merged in version control with conflicts in the marker area | Resolve the merge conflict. Ensure exactly one BEGIN and one END marker remain. Re-run `carto patterns` to regenerate clean content. |
| A different tool uses similar markers | Ensure no other tool uses the exact string `<!-- BEGIN CARTO INDEX -->` or `<!-- END CARTO INDEX -->`. If conflicts exist, the other tool's markers should use different text. |

### Manual Fix

Edit the file to have exactly this structure:

```
... your custom content above ...

<!-- BEGIN CARTO INDEX -->
... Carto-generated content (will be replaced on next generation) ...
<!-- END CARTO INDEX -->

... your custom content below ...
```

Then run `carto patterns /path/to/project` to regenerate clean content between the markers.

---

## Symptom: User Content Outside Markers Was Overwritten

Content that was placed before or after the Carto markers was lost after a skill file generation.

### Diagnostic Steps

1. This should not happen under normal operation. The marker-based injection explicitly preserves content outside the markers.

2. Check version control for the file's history:
   ```bash
   git log --oneline -- CLAUDE.md
   git diff HEAD~1 -- CLAUDE.md
   ```

3. Check if the file was replaced entirely (no markers in the previous version, and the new version has only markers + generated content).

4. Check if a different tool or process overwrote the file.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Bug in marker detection (possible but unlikely) | Restore the file from version control. File a bug report with the before and after file contents. |
| Another tool or script overwrote the file | Identify the tool. Ensure it does not conflict with Carto's file writes. |
| File had no markers, and Carto created a new file instead of appending | Check the logic: if the file existed but had no markers, content should be appended, not replaced. If this did not happen, file a bug. |
| Manual error (wrong file path) | Verify the project root path is correct. |

### Prevention

- Commit skill files to version control so they can be restored.
- Place custom content clearly outside the markers.
- Verify marker integrity after merge operations.

---

## Symptom: Skill File Content Is Empty or Minimal

The file exists with markers, but the content between them is very short or missing key sections (no modules, no patterns).

### Diagnostic Steps

1. Check synthesis data in Memories:
   ```bash
   # Check blueprint
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories?source=carto/{project}/layer:blueprint&limit=1"

   # Check patterns
   curl -s -H "X-API-Key: $MEMORIES_API_KEY" "$MEMORIES_URL/memories?source=carto/{project}/layer:patterns&limit=1"
   ```

2. If the data exists but is sparse, the deep analysis may have produced minimal results (small project, few patterns detected).

3. If the data is missing, Phase 4 or Phase 5 may have partially failed.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Deep analysis produced minimal synthesis | Expected for very small projects. The content reflects what the LLM found. |
| Phase 4 partially failed (some modules analyzed, others not) | Check Phase 4 errors. Re-index with `--full`. |
| Phase 5 partially failed (some data not stored) | Check Phase 5 errors. Verify Memories connectivity. Re-index. |
| Wrong project name (pulling data for a different or nonexistent project) | Verify `--project` matches the name used during indexing. |

---

## Symptom: Active Index Instructions Not Working

AI assistants cannot query or write back to Memories using the curl commands in the skill file.

### Diagnostic Steps

1. Check if the Memories server is running: `curl -s $MEMORIES_URL/health`
2. Verify the curl commands in the skill file use the correct `MEMORIES_URL` and `MEMORIES_API_KEY` values.
3. Check if the environment variables referenced in the skill file are set in the AI assistant's environment.

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| Memories server is not running | Start the Memories server. |
| `MEMORIES_URL` or `MEMORIES_API_KEY` changed since generation | Regenerate skill files: `carto patterns /path/to/project`. |
| AI assistant does not have the environment variables set | Ensure the assistant's shell environment includes `MEMORIES_URL` and `MEMORIES_API_KEY`. |
| Firewall blocks the assistant's access to Memories | Check network access from the assistant's runtime environment. |

---

## Escalation

Escalate when:

- User content is overwritten despite being outside markers (confirmed via version control diff). This indicates a bug in the marker detection or injection logic.
- Markers are silently removed or corrupted by the generation process itself (not by external tools or merge conflicts).
- The generated content is structurally malformed (broken markdown, incomplete sections) despite valid synthesis data in Memories.
