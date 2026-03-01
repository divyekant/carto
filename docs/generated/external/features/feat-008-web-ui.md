# Web Dashboard

Carto includes a built-in web dashboard that lets you manage projects, trigger indexing, query the semantic graph, and configure settings — all from your browser. The UI is a React application embedded directly in the Carto binary, so there is nothing extra to install.

## Getting Started

Start the Carto server:

```bash
carto serve --port 8950
```

Then open your browser to **http://localhost:8950**. You will land on the Dashboard page.

## Pages

### Dashboard

The dashboard is your home screen. It shows a table of all known projects with key information at a glance:

- **Project name** and path
- **Last indexed** timestamp
- **File count** and indexing status

From here you can click into any project for more detail, or add a new project directly.

### Index

The Index page lets you trigger indexing for any project. Select a project, choose between incremental or full indexing, and click **Start**. A progress indicator shows each phase of the pipeline in real time:

1. Scanning files
2. Chunking and extracting atoms
3. Running deep analysis
4. Storing results in Memories
5. Generating skill files

You can watch the entire run or navigate away — indexing continues in the background.

### Query

The Query page gives you a search interface over your indexed codebases. Type a natural-language question, select a project, and optionally choose a retrieval tier (`mini`, `standard`, or `full`). Results include an answer grounded in the codebase along with references to the relevant source files.

### Project Detail

Click on any project from the Dashboard to open its detail view. This is a two-column layout:

- **Left column** — Project metadata, indexing history, and discovered modules
- **Right column** — A sources editor where you can add or update external signal sources (CI pipelines, issue trackers, documentation URLs) that Carto incorporates during indexing

Changes to sources are saved immediately and take effect on the next indexing run.

### Settings

The Settings page lets you configure integrations and global options:

- **LLM provider** — Choose between Anthropic, OpenAI, or Ollama
- **API keys** — Set your LLM API key
- **Memories URL** — Point Carto at your Memories server
- **Projects directory** — Default base path for discovering codebases

## Configuration

The server accepts two flags that affect the web UI:

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8950` | Port the server (and UI) listens on |
| `--projects-dir` | current directory | Base directory for project discovery |

Example:

```bash
carto serve --port 9000 --projects-dir /home/dev/repos
```

## Examples

### Browse and Index a Project

1. Open **http://localhost:8950**
2. On the Dashboard, click **Add Project**
3. Enter a name and the path to your codebase
4. Click into the project, then click **Index**
5. Watch the progress bar as Carto scans and analyzes the code

### Query the Index

1. Navigate to the **Query** page
2. Select your project from the dropdown
3. Type a question: "How is the database connection pool configured?"
4. Review the answer and linked source files

### Configure Sources for a Project

1. Open a project from the Dashboard
2. In the right column, find the **Sources** editor
3. Add entries like:
   - CI: `https://github.com/your-org/your-repo/actions`
   - Issues: `https://github.com/your-org/your-repo/issues`
4. Sources are saved automatically
5. Re-index the project to incorporate the new signal data

### Update Global Settings

1. Navigate to **Settings**
2. Update your LLM provider or API key
3. Changes take effect immediately for subsequent operations

## Limitations

- **Requires a running server** — The web UI is served by `carto serve`. It is not a standalone application.
- **Embedded in the binary** — The React SPA is compiled into the Carto binary during build. You do not need Node.js or npm at runtime, but modifying the UI requires rebuilding Carto.
- **Local access** — By default the server binds to localhost. To access it from another machine, configure your network or use a reverse proxy.

## Related

- [CLI Reference](feat-006-cli.md) — perform the same operations from the command line
- [REST API](feat-007-rest-api.md) — the HTTP endpoints that power this dashboard
- [Docker Deployment](feat-010-docker-deployment.md) — run the server in a container
