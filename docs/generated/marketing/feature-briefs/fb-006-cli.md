---
id: fb-006
type: feature-brief
audience: marketing
topic: Command-Line Interface
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Command-Line Interface

## One-Liner

Index, query, and manage your codebase intelligence from a single CLI with 9 purpose-built commands.

## Problem

Managing code understanding tools shouldn't require a GUI. Developers live in the terminal. They need a tool that meets them where they work — one that scripts, pipes, and automates without friction. Every context switch to a browser is lost momentum.

## Solution

Carto ships as a single binary with 9 commands that cover the entire workflow: scanning codebases, building intelligence indexes, querying stored context, and generating skill files for AI assistants. Every command supports `--json` output for scripting and automation.

## Key Benefits

- **Single binary, zero dependencies.** Download it, run it. No installers, no runtimes, no package managers.
- **9 commands, full coverage.** Index a codebase, query its intelligence, manage projects, generate skill files — all from the terminal.
- **Scriptable by design.** Every command supports `--json` output. Pipe Carto into `jq`, feed it into CI steps, or wrap it in your own scripts.
- **CI/CD friendly.** Runs headless, exits with meaningful status codes, and produces machine-readable output. Drop it into any pipeline.
- **Incremental by default.** Re-indexing only processes changed files. Fast enough to run on every commit.

## Who This Is For

- **Primary:** Developers who prefer the terminal for daily work.
- **Secondary:** DevOps and platform engineers building CI/CD pipelines that need codebase context.
- **Tertiary:** Teams automating code analysis across multiple repositories.

## Suggested Messaging

- "One binary. Nine commands. Complete codebase intelligence at your fingertips."
- "Carto fits your workflow — not the other way around. Script it, pipe it, automate it."
- "From `carto index` to `carto query` in seconds. No GUI required."
- "CI/CD pipelines deserve codebase intelligence too. Carto's CLI was built for automation."
- "Ship a single binary to your team and every developer has instant codebase context."

## Competitive Differentiators

- Unlike browser-based code analysis tools, Carto runs where developers already work.
- JSON output means Carto integrates with any toolchain — no vendor lock-in.
- Single binary distribution eliminates the "works on my machine" problem.
