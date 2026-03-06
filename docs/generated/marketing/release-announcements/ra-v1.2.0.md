---
id: ra-v1.2.0
type: release-announcement
audience: marketing
version: 1.2.0
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Carto v1.2.0: Your AI Agents Just Learned to Drive

AI agents run CLI commands on your behalf thousands of times a day. Until now, they had to scrape colored text, dodge spinners, and guess at error formats. Carto v1.2.0 changes the game: a fully agent-native CLI that auto-detects its consumer and delivers structured output -- no flags, no parsing, no guesswork.

And for developers at the terminal? The experience just got better too.

## The Agent-Native Story

Every Carto command now speaks two languages. When an AI agent calls Carto -- through a pipe, a subprocess, or any non-interactive context -- it receives clean JSON wrapped in a predictable envelope with typed error codes. When a developer runs the same command in their terminal, they get gold-branded colored output with progress spinners and clear formatting.

This is not a `--json` flag. This is automatic detection. Your agents and your team use the same commands, and each gets the output format designed for them.

## Six New Commands

**`carto init`** -- An interactive setup wizard that walks you through project configuration. Answer a few questions and your project is ready to index.

**`carto completions`** -- Generate shell autocompletion scripts for bash, zsh, fish, or PowerShell. Tab-complete every command and flag.

**`carto export`** -- Export your entire codebase index as NDJSON. Back it up, share it with teammates, or migrate it to another environment.

**`carto import`** -- Import a previously exported index. Restore from backup, onboard a new machine, or collaborate across teams.

**`carto logs`** -- Query the structured audit trail. See who indexed what, when commands ran, and what changed -- all in a queryable format.

**`carto upgrade`** -- Check for new versions and self-update in place. No package managers, no manual downloads, no version drift across your team.

## What Else Improved

- **Typed error codes** across every command, so agents and scripts can handle failures programmatically instead of parsing error strings.
- **Confirmation guards** on destructive operations prevent accidental data loss in interactive sessions while auto-confirming in non-interactive (agent) contexts.
- **Profile support** lets you maintain multiple configurations for different projects, environments, or LLM providers.
- **42+ new tests** with race detection ensure the expanded CLI is rock-solid.

## Zero Breaking Changes

Every existing command, flag, script, and CI pipeline works exactly as it did before. The v1.2.0 overhaul is purely additive. Upgrade with confidence.

## What's Next

- **Watch mode** -- Continuous indexing that re-analyzes on every file save, keeping your AI assistants perpetually up to date.
- **Team sync** -- Shared indexes that let your entire engineering organization work from a single source of codebase intelligence.
- **Plugin system** -- Extend Carto's CLI with custom commands tailored to your team's workflow.

---

Carto v1.2.0 is available now. Upgrade with `carto upgrade` and give your AI agents the CLI they deserve.
