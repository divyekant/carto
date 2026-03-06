---
id: op-006
type: one-pager
audience: marketing
topic: CLI & Agent Usability
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Carto CLI: The First Agent-Native Interface for Codebase Intelligence

## The Problem

AI agents are the fastest-growing consumer of developer CLIs. Claude, Cursor, and Copilot run terminal commands millions of times a day on behalf of developers. But every CLI they encounter was designed for human eyes -- colored text, progress spinners, unpredictable error messages. Agents have to scrape output, guess at formats, and fail silently when things go wrong.

Developers shouldn't have to choose between a CLI that works for their AI assistants and one that works for them.

## The Solution

Carto's CLI auto-detects who is calling it and adapts. Run a command in your terminal and you get gold-branded colored output with progress spinners. Pipe that same command through an AI agent and it receives clean, structured JSON with typed error codes -- automatically. No flags. No configuration.

This is not a feature toggle. It is a fundamental design principle: every command, every output, every error is built to serve both humans and machines from the ground up.

## Why It Matters

**For AI-assisted teams:** Your agents can drive Carto without custom parsing, wrapper scripts, or fragile regex. Index a codebase, query its intelligence, export data -- all through structured commands that agents understand natively.

**For developers:** 18 commands cover the complete workflow. An interactive setup wizard gets you running in seconds. Shell completions eliminate typos. A built-in upgrade command keeps your installation current.

**For platform teams:** Typed error codes and structured audit logging give you the observability and compliance controls that enterprise environments demand. Export and import make codebase intelligence portable across machines, teams, and environments.

**For existing users:** Zero breaking changes. Every script, pipeline, and workflow you have today continues working exactly as before.

## Key Capabilities

| Capability | What It Delivers |
|---|---|
| **Auto-adaptive output** | JSON for agents, rich terminal UI for humans -- detected automatically |
| **18 CLI commands** | Full workflow coverage: index, query, export, import, upgrade, audit, and more |
| **Interactive setup** | `carto init` wizard configures your project in seconds |
| **Shell completions** | Tab-complete every command and flag in bash, zsh, fish, and PowerShell |
| **Data portability** | Export/import codebase indexes as NDJSON -- back up, share, or migrate freely |
| **Structured audit log** | Every operation logged with queryable structured entries |
| **Self-update** | `carto upgrade` checks for and installs the latest version |

## Get Started

Install the Carto binary. Run `carto init` to configure your first project. Run `carto index` to build your codebase intelligence. Your AI assistants -- and your team -- are ready to go.
