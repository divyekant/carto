# Use Case: Web Dashboard Usage

**ID:** uc-008
**Topic:** Web Dashboard Usage
**Trigger:** A user opens `http://localhost:8950` in a browser to manage projects, trigger indexing, and query the semantic index through the Web UI.

---

## Primary Flow

### 1. Start the Server

```bash
carto serve --port 8950
```

The server starts and serves both the REST API and the embedded Web UI.

### 2. Open the Dashboard

Navigate to `http://localhost:8950` in a browser. The Dashboard page loads, displaying a table of all registered projects. Each row shows the project name, path, last indexed timestamp, and module count.

### 3. Select a Project

Click on a project row to navigate to the Project Detail page. The detail view shows:
- **Left column:** Project metadata (name, path, status, last indexed time, module list).
- **Right column:** Sources editor for configuring external signal sources.

### 4. Trigger Indexing

Navigate to the Index page (via the sidebar). Select the target project from the dropdown, optionally choose a specific module or toggle full re-index, and click the index button.

The UI opens an SSE connection to the server. Live progress is displayed:
- Current pipeline phase (scan, atoms, history, signals, deep analysis, store, skill files).
- Progress percentage and status messages.
- A stop button to cancel the operation.

### 5. Monitor Progress

The SSE stream delivers real-time updates. Each phase transition updates the progress indicator. On completion, the UI shows a success message with the total duration. On error, the error message and the failed phase are displayed.

### 6. Query the Index

Navigate to the Query page. Enter a natural language question, select the project, choose a retrieval tier (`mini`, `standard`, or `full`), and submit. Results appear below the query form, showing relevant context from the semantic index.

---

## Variation: First-Time Setup

On first visit, the Dashboard is empty (no projects registered).

1. Use the CLI or API to register a project:
   ```bash
   curl -X POST http://localhost:8950/api/projects \
     -H "Content-Type: application/json" \
     -d '{"name": "myapp", "path": "/path/to/myapp"}'
   ```

2. Refresh the Dashboard. The new project appears in the table.

3. Navigate to the Index page and trigger the first index.

Currently, the Web UI does not provide a project creation form; projects are registered through the API or CLI.

---

## Variation: Re-Indexing After Code Changes

1. Open the Index page.
2. Select the project.
3. Leave "full re-index" unchecked (incremental is the default). The pipeline only processes files that changed since the last index, based on the SHA-256 manifest.
4. Click to start indexing. The SSE progress stream shows which files are being re-processed.
5. On completion, the updated data is immediately available for queries.

---

## Variation: Querying Without Indexing

If a project has already been indexed (by the CLI, API, or a previous UI session), the user can go directly to the Query page and submit questions. No indexing step is required.

---

## Variation: Configuring Settings

1. Navigate to the Settings page via the sidebar.
2. View current configuration: LLM provider, model selections, Memories URL.
3. Edit values in the two-column grid form.
4. Save. The `PUT /api/config` call updates the server's runtime configuration.

---

## Postconditions

- Projects are visible on the Dashboard with up-to-date status.
- Indexing has been triggered and completed (or cancelled) with live progress feedback.
- The query interface returns results from the current semantic index.
- Configuration changes take effect for subsequent operations.
