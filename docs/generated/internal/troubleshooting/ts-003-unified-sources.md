---
id: ts-003
type: troubleshooting
audience: internal
topic: Source Fetching Failures
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Troubleshooting: Source Fetching Failures

## Quick Check

1. **Credentials are set:** Verify the relevant environment variable is non-empty for each source you expect to be active.
2. **Per-project config exists (if used):** Check for `sources.yaml` in the project root or `.carto/` directory.
3. **External service is reachable:** Test connectivity to the service's API endpoint from the machine running Carto.

---

## Symptom: "0 signals collected"

No signals were produced by any source during Phase 3.

### Diagnostic Steps

1. Check if any source credentials are configured:
   ```bash
   echo "GITHUB_TOKEN: ${GITHUB_TOKEN:+(set)}"
   echo "JIRA_URL: ${JIRA_URL:+(set)}"
   echo "LINEAR_TOKEN: ${LINEAR_TOKEN:+(set)}"
   echo "NOTION_TOKEN: ${NOTION_TOKEN:+(set)}"
   echo "SLACK_TOKEN: ${SLACK_TOKEN:+(set)}"
   ```

2. If no credentials are set, only the Git source is active. Check if the project has a git repository:
   ```bash
   git -C /path/to/project log --oneline -5
   ```

3. If credentials are set, check `Result.Errors` for source-specific failures.

4. Check if `sources.yaml` exists and is valid YAML:
   ```bash
   cat /path/to/project/.carto/sources.yaml 2>/dev/null || cat /path/to/project/sources.yaml 2>/dev/null || echo "No sources.yaml found"
   ```

### Root Causes & Resolutions

| Root Cause | Resolution |
|---|---|
| No source credentials configured | Set environment variables for desired sources. Only Git runs by default. |
| All configured sources failed | Check each source individually (see source-specific sections below). |
| `sources.yaml` has invalid syntax | Fix YAML syntax errors. Use a YAML linter to validate. |
| Project is not a git repository | Git source produces nothing for non-git directories. Other sources may still work. |
| All source API calls returned empty results | Valid outcome -- the project may have no issues, no docs, etc. |

---

## Symptom: GitHub Source Failures

### "401 Bad credentials"

**Cause:** `GITHUB_TOKEN` is invalid, expired, or revoked.

**Resolution:**
1. Verify the token: `curl -s -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user`
2. If 401, generate a new token from GitHub Settings > Developer settings > Personal access tokens.
3. Ensure the token has `repo` scope (for private repos) or `public_repo` (for public only).

### "404 Not Found" for repository

**Cause:** The `repo` config does not match any repository, or the token lacks access to that repo.

**Resolution:**
1. Verify the repo identifier in `sources.yaml` matches `owner/repo` format exactly.
2. If no `sources.yaml`, check that the git remote URL is a GitHub URL that the token can access.
3. For private repos, ensure the token has `repo` scope.

### "403 rate limit exceeded"

**Cause:** GitHub API rate limit hit (5,000 requests/hour for authenticated users).

**Resolution:** Wait for the rate limit to reset (check `X-RateLimit-Reset` header). Reduce `max_results` in the source config. Avoid running multiple indexing jobs simultaneously.

---

## Symptom: Jira Source Failures

### "401 Unauthorized"

**Cause:** `JIRA_TOKEN` or `JIRA_EMAIL` is incorrect.

**Resolution:**
1. Verify credentials: `curl -s -u "$JIRA_EMAIL:$JIRA_TOKEN" "$JIRA_URL/rest/api/3/myself"`
2. Regenerate the API token from https://id.atlassian.com/manage-profile/security/api-tokens.
3. Ensure `JIRA_EMAIL` matches the Atlassian account email (not a different email).

### "404 Project not found"

**Cause:** `project_key` in `sources.yaml` does not match any Jira project.

**Resolution:** Verify the project key in Jira (visible in the project URL: `https://company.atlassian.net/browse/PROJ`). Project keys are case-sensitive.

### Connection failures to on-premise Jira

**Cause:** `JIRA_URL` points to an internal Jira instance that is not reachable from the machine running Carto.

**Resolution:** Verify network access to the Jira URL. Check VPN/proxy requirements. Ensure the URL includes the correct protocol and port.

---

## Symptom: Linear Source Failures

### "401 Authentication required"

**Cause:** `LINEAR_TOKEN` is invalid.

**Resolution:**
1. Generate a new API key from Linear Settings > API.
2. Verify: `curl -s -H "Authorization: $LINEAR_TOKEN" -H "Content-Type: application/json" -d '{"query":"{ viewer { id } }"}' https://api.linear.app/graphql`

### "Team not found" or empty results

**Cause:** The `team` value in `sources.yaml` does not match a Linear team slug.

**Resolution:** Check the team identifier in Linear (Settings > Teams). Use the team slug, not the display name.

---

## Symptom: Notion Source Failures

### "401 Unauthorized"

**Cause:** `NOTION_TOKEN` is invalid or the integration has been revoked.

**Resolution:**
1. Verify the token: `curl -s -H "Authorization: Bearer $NOTION_TOKEN" -H "Notion-Version: 2022-06-28" https://api.notion.com/v1/users/me`
2. Re-create the integration from https://www.notion.so/my-integrations if needed.

### "Object not found" for database or page

**Cause:** The integration has not been shared with the target database/page.

**Resolution:** In Notion, open the target database or page, click "Share", and add the integration by name. The integration must be explicitly granted access to each page or database.

---

## Symptom: Slack Source Failures

### "invalid_auth" or "not_authed"

**Cause:** `SLACK_TOKEN` is invalid or the bot has been deactivated.

**Resolution:**
1. Verify the token: `curl -s -H "Authorization: Bearer $SLACK_TOKEN" https://slack.com/api/auth.test`
2. Check that the Slack app is still installed in the workspace.
3. Regenerate the bot token from the Slack API dashboard if needed.

### "channel_not_found"

**Cause:** The channel ID in `sources.yaml` is incorrect, or the bot is not a member of the channel.

**Resolution:**
1. Use the channel ID (starts with `C`), not the channel name. Find the ID in Slack by right-clicking the channel > "View channel details" > scroll to the bottom.
2. Invite the bot to the channel: `/invite @botname` in the channel.

### "missing_scope"

**Cause:** The bot token lacks the required `channels:history` scope.

**Resolution:** Add the `channels:history` scope in the Slack app configuration (OAuth & Permissions), then reinstall the app to the workspace.

---

## Symptom: PDF Source Failures

### "no matching files"

**Cause:** The glob pattern in `sources.yaml` does not match any files.

**Resolution:**
1. Check the path pattern. Paths are relative to the project root.
2. Test the glob: `ls /path/to/project/docs/specs/*.pdf`
3. Ensure the PDF files exist and are readable.

### "failed to extract text"

**Cause:** The PDF file is corrupted, password-protected, or uses image-only content (scanned documents without OCR).

**Resolution:** Verify the PDF can be opened and read normally. Password-protected PDFs are not supported. Image-only PDFs will not yield text.

---

## Symptom: Web Source Failures

### "connection refused" or "timeout"

**Cause:** The URL is unreachable from the machine running Carto.

**Resolution:** Verify the URL is accessible: `curl -s -o /dev/null -w "%{http_code}" "https://docs.example.com"`. Check for firewall, proxy, or DNS issues.

### "403 Forbidden"

**Cause:** The website blocks automated scraping.

**Resolution:** Some sites block requests without a browser-like User-Agent or use bot detection. The web source may not be able to access all URLs. Consider using a different data source for that content.

---

## Escalation

Escalate when:

- A source consistently fails with errors not covered above.
- Source API behavior has changed (new auth requirements, endpoint changes).
- The concurrent fetching mechanism itself appears to hang or deadlock (all sources fail to complete).
- Credential validation succeeds but `FetchSignals()` produces zero results for a source that should have data.
