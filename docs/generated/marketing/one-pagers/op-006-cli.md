---
id: op-006
type: one-pager
audience: marketing
topic: Command-Line Interface
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Carto CLI: Codebase Intelligence From Your Terminal

## The Problem

Developers don't want another dashboard. They want tools that fit the way they already work — in the terminal, in scripts, in CI pipelines. Most code analysis tools force you into a browser, breaking your flow every time you need context.

## The Solution

Carto ships as a single binary with 9 purpose-built commands that give you complete control over codebase intelligence from the command line. Index a project, query its context, generate skill files for AI assistants — all without leaving your terminal.

## Key Benefits

**Ship it anywhere.** One binary, no dependencies. Works on macOS, Linux, and in containers. Hand it to a new developer and they're productive in minutes.

**Automate everything.** Every command supports `--json` output. Feed Carto into `jq`, pipe results into downstream tools, or embed it in shell scripts. It was built for automation from day one.

**Run it in CI/CD.** Meaningful exit codes, headless operation, and machine-readable output mean Carto drops into any pipeline. Keep your codebase intelligence fresh on every push.

**Incremental speed.** Re-indexing only touches changed files. Run it on every commit without slowing down your workflow.

**Full workflow coverage.** Nine commands handle scanning, indexing, querying, skill file generation, and project management. No feature is locked behind a GUI.

## How It Works

Install the binary. Point it at your codebase. Run `carto index`. That's it — your codebase intelligence is built and ready to query. Use `carto query` to retrieve structured context, or `carto generate` to produce skill files that supercharge your AI coding assistants.

## Who It's For

Developers who live in the terminal. DevOps engineers building intelligent pipelines. Platform teams automating code analysis across dozens of repositories.

## Get Started

Download the Carto binary, set your API key, and run your first index in under a minute. Visit the Quick Start guide to begin.
