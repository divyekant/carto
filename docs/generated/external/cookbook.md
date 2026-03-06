---
type: cookbook
audience: external
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Cookbook

Practical recipes for common Carto tasks. Each recipe is self-contained -- copy the commands and adapt them to your environment.

---

## Export and Re-Import a Project Index

You can back up a project's entire index as NDJSON (one JSON object per line) and restore it on the same or a different machine.

### Back up an index

```bash
carto export --project my-app > my-app-backup.ndjson
```

This streams every indexed entry to the file. You can also export a specific layer:

```bash
carto export --project my-app --layer atoms > my-app-atoms.ndjson
```

### Restore an index (append)

To add the exported data into an existing index without removing what's already there:

```bash
cat my-app-backup.ndjson | carto import --project my-app
```

### Restore an index (full replace)

To wipe the existing index and replace it entirely with the backup:

```bash
carto import --project my-app --strategy replace --yes < my-app-backup.ndjson
```

The `--yes` flag skips the confirmation prompt. Without it, you'll be asked to confirm the deletion of existing data.

### Migrate an index between Memories servers

Export from one environment and import into another by pointing `MEMORIES_URL` to the target:

```bash
# Export from source
MEMORIES_URL=http://source-server:8900 carto export --project my-app > my-app.ndjson

# Import into target
MEMORIES_URL=http://target-server:8900 carto import --project my-app --strategy replace --yes < my-app.ndjson
```

---

## Use Carto in CI/CD with JSON Parsing

Carto automatically emits JSON when its output is piped, so you can parse results with `jq` in any CI/CD pipeline.

### GitHub Actions: Index on push and check status

```yaml
name: Carto Index
on: [push]

jobs:
  index:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Index codebase
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          MEMORIES_URL: ${{ secrets.MEMORIES_URL }}
        run: |
          RESULT=$(carto index --path . --name ${{ github.repository }})
          echo "Indexing result: $(echo "$RESULT" | jq -r '.ok')"

      - name: Generate skill files
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          MEMORIES_URL: ${{ secrets.MEMORIES_URL }}
        run: carto patterns --project ${{ github.repository }}
```

### Check for errors in a script

```bash
#!/bin/bash
set -e

RESULT=$(carto index --path /repos/my-app --name my-app 2>&1)
OK=$(echo "$RESULT" | jq -r '.ok')

if [ "$OK" != "true" ]; then
  ERROR=$(echo "$RESULT" | jq -r '.error')
  CODE=$(echo "$RESULT" | jq -r '.code')
  echo "Indexing failed: $ERROR (code: $CODE)"
  exit 1
fi

echo "Indexed successfully"
```

### Use exit codes for error handling

Each error type has a specific exit code you can check:

```bash
carto status --project my-app
EXIT_CODE=$?

case $EXIT_CODE in
  0) echo "Success" ;;
  2) echo "Project not found" ;;
  3) echo "Configuration error" ;;
  4) echo "Connection error -- is Memories running?" ;;
  5) echo "Authentication failed -- check your API key" ;;
  *) echo "Unknown error (exit $EXIT_CODE)" ;;
esac
```

### Parse query results in a pipeline

```bash
# Extract just the answer text
carto query --project my-app "What are the API endpoints?" | jq -r '.data.answer'

# Check if an upgrade is available
UPDATE=$(carto upgrade --check | jq -r '.data.update_available')
if [ "$UPDATE" = "true" ]; then
  echo "A new version of Carto is available"
fi
```

---

## Set Up Shell Completions

Tab-completion makes working with Carto faster. Choose your shell:

### Bash

Add to your `~/.bashrc` (or `~/.bash_profile` on macOS):

```bash
# Load Carto completions
source <(carto completions bash)
```

Then reload your shell:

```bash
source ~/.bashrc
```

### Zsh

Generate the completion file and place it in your `fpath`:

```bash
carto completions zsh > "${fpath[1]}/_carto"
```

Then restart your shell or run:

```bash
compinit
```

### Fish

Save the completion file:

```bash
carto completions fish > ~/.config/fish/completions/carto.fish
```

Fish picks it up automatically on the next shell session.

### PowerShell

Add to your PowerShell profile:

```powershell
carto completions powershell | Out-String | Invoke-Expression
```

### Verify completions work

After setup, type `carto ` and press Tab. You should see a list of available commands. Type `carto index --` and press Tab to see available flags.

---

## Use `carto init` for Automated Setup

The `carto init` command supports both interactive and non-interactive modes, making it suitable for both humans and automation.

### Interactive setup (humans)

```bash
carto init
```

The wizard prompts for each value with sensible defaults:

```
Carto Init

  This wizard will set up your Carto configuration.
  Press Enter to accept the default value shown in [brackets].

  LLM provider (anthropic, openai, ollama) [anthropic]:
  API key [sk-a...xxxx]:
  Memories server URL [http://localhost:8900]:
  Memories API key:
  Projects directory [/home/user/projects]:

Configuration written to /home/user/.config/carto/config.json

  LLM provider:         anthropic
  API key:              sk-a...xxxx
  Memories URL:         http://localhost:8900

  Run carto doctor to verify your setup.
```

### Non-interactive setup (automation)

For Docker entrypoints, CI/CD, or scripted provisioning:

```bash
carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "$ANTHROPIC_API_KEY" \
  --memories-url "$MEMORIES_URL" \
  --memories-key "$MEMORIES_API_KEY" \
  --projects-dir /data/projects
```

In a Dockerfile:

```dockerfile
RUN carto init --non-interactive \
  --llm-provider anthropic \
  --api-key "${ANTHROPIC_API_KEY}" \
  --memories-url "http://memories:8900"
```

### Verify after setup

Always run `carto doctor` after `carto init` to confirm everything is wired up correctly:

```bash
carto doctor
```

---

## Query Audit Logs for Errors

The audit log records every CLI command execution with its result, making it useful for debugging and monitoring.

### Enable audit logging

Set the log file path:

```bash
export CARTO_AUDIT_LOG=/var/log/carto-audit.log
```

Or pass it per-command:

```bash
carto index --path . --name my-app --log-file /var/log/carto-audit.log
```

### View recent entries

```bash
# Show the 20 most recent entries (default)
carto logs

# Show the last 50 entries
carto logs --last 50
```

### Filter by command

```bash
# Show only index operations
carto logs --command index

# Show only query operations
carto logs --command query
```

### Filter by result

```bash
# Show only errors
carto logs --result error

# Show only successes
carto logs --result ok
```

### Combine filters

```bash
# Show the last 10 failed index runs
carto logs --command index --result error --last 10
```

### Tail the log in real time

Watch for new events as they happen (useful when monitoring a running instance):

```bash
carto logs --follow
```

Press Ctrl+C to stop tailing.

### Get log data as JSON

For programmatic processing:

```bash
carto logs --last 100 --json | jq '.data.entries[] | select(.result == "error")'
```

---

## End-to-End: New Project Setup

A complete recipe for setting up Carto with a new project from scratch:

```bash
# 1. Configure (first time only)
carto init

# 2. Verify environment
carto doctor

# 3. Index your project
carto index --path /path/to/my-project --name my-project

# 4. Check status
carto status --project my-project

# 5. Ask a question
carto query --project my-project "What is the overall architecture?"

# 6. Generate skill files for AI assistants
carto patterns --project my-project

# 7. Start the web UI
carto serve --port 8950
```

---

## Managing Multiple Projects

You can index and query as many codebases as you like. Each project is stored under its own name:

```bash
# Index multiple projects
carto index --path /repos/frontend --name frontend
carto index --path /repos/backend  --name backend
carto index --path /repos/infra    --name infra

# List all projects
carto projects list

# Query each one independently
carto query --project frontend "How is routing configured?"
carto query --project backend  "Where is the auth middleware?"
carto query --project infra    "What Terraform modules are used?"

# Generate skill files for each
carto patterns --project frontend
carto patterns --project backend
```

---

## Periodic Index Refresh with Cron

Keep your index up to date by running a cron job:

```bash
# Add to crontab (crontab -e)
# Re-index every night at 2am
0 2 * * * ANTHROPIC_API_KEY=sk-ant-... MEMORIES_URL=http://localhost:8900 /usr/local/bin/carto index --path /repos/my-app --name my-app --log-file /var/log/carto-audit.log
```

Carto uses incremental indexing by default, so only changed files are re-processed. This makes nightly runs fast and cost-effective.
