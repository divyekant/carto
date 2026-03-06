---
id: fb-006
type: feature-brief
audience: marketing
topic: CLI & Agent Usability
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: CLI & Agent Usability

## One-Liner

Carto's CLI speaks both human and machine -- AI agents get structured JSON automatically, developers get a beautiful terminal experience.

## Problem

The rise of AI-assisted development created a new consumer of developer tools: the AI agent. Claude, Cursor, and Copilot now run CLI commands on behalf of developers every day. But every existing CLI was designed for human eyes only -- colored text, spinners, interactive prompts. Agents have to scrape, guess, and hope.

On the other side, developers still want tools that feel good in the terminal. Rich output, clear feedback, smooth workflows. The industry treats these as opposing goals. We don't.

## Solution

Carto's overhauled CLI auto-detects who is calling it. When a human runs a command in a terminal, they get gold-branded colored output with progress spinners. When an AI agent pipes a command or runs it non-interactively, it gets clean structured JSON with typed error codes and predictable envelopes. No flags to set. No configuration. It just works.

The overhaul also adds 9 new commands (18 total), shell completions, data portability, structured audit logging, and self-update -- turning Carto's CLI into a complete platform interface.

## Key Benefits

- **Agent-native by default.** AI agents get structured JSON output automatically. No `--json` flag, no parsing gymnastics. Carto detects the consumer and adapts.
- **Developer-friendly always.** Terminal users get interactive wizards, shell completions, gold-branded colored output, and progress spinners. The CLI feels as polished as a GUI.
- **Data portable.** Export your entire codebase index as NDJSON. Import it on another machine, share it with your team, or back it up. Your intelligence data is yours.
- **Enterprise audit trail.** Every command writes a structured log entry. Query your audit trail to see who indexed what, when, and what changed. Compliance teams will thank you.
- **Self-maintaining.** Built-in upgrade checks and a single `carto upgrade` command keep every installation current. No package managers, no manual downloads.
- **Zero breaking changes.** Every existing workflow, script, and CI pipeline continues working exactly as before. The overhaul is purely additive.

## Who This Is For

- **Primary:** AI agents and AI-assisted developers. Claude Code, Cursor, and Copilot users who want their AI assistants to drive Carto without friction.
- **Secondary:** Developers who live in the terminal and want a polished, productive CLI experience.
- **Tertiary:** Platform and DevOps teams embedding Carto in CI/CD pipelines, automation scripts, and custom toolchains.

## By the Numbers

- 18 CLI commands (doubled from 9)
- 42+ new tests, all passing with race detection
- 35 files changed, 3,199 lines added
- Zero breaking changes

## Suggested Messaging

- "Your AI agents already use your CLI. Carto is the first CLI built for them."
- "Carto auto-detects human or machine and adapts its output. No flags. No config. It just works."
- "18 commands. Two output modes. One CLI that serves every consumer."
- "Export your codebase intelligence. Import it anywhere. Your data, your way."
- "The first agent-native CLI for codebase intelligence."

## Competitive Differentiators

- Most developer tools require `--json` flags and manual output parsing. Carto auto-detects and adapts -- agents get structured JSON without asking.
- No competing codebase intelligence tool offers data portability via export/import.
- Built-in upgrade and audit logging remove operational overhead that other CLIs push onto users.
- Shell completions and interactive wizards match the usability of dedicated GUI tools, without leaving the terminal.
