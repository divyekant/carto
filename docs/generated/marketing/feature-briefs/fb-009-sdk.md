---
id: fb-009
type: feature-brief
audience: marketing
topic: Go SDK
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Go SDK

## One-Liner

Embed codebase intelligence directly into your Go applications with a thin, well-tested SDK.

## Problem

Building custom developer tools that need code understanding is hard. You end up shelling out to CLI commands, parsing text output, and managing process lifecycles. It's fragile, slow, and painful to maintain. Teams building internal platforms deserve a proper programmatic interface.

## Solution

Carto provides a Go SDK with three core functions — Index, Query, and Sources — that let you embed codebase intelligence directly into your Go applications. It's the same battle-tested code that powers the Carto CLI, exposed as a clean, importable package.

## Key Benefits

- **Three functions, complete power.** `Index` builds codebase intelligence, `Query` retrieves it, and `Sources` manages project configurations. Simple surface area, deep capability.
- **Battle-tested code.** The SDK is not a wrapper around the CLI. It's the same core engine the CLI uses. Every feature you see in the CLI runs through this code.
- **Go-native design.** Proper error handling, context support for cancellation, and idiomatic Go patterns. It feels like part of your codebase, not an external dependency.
- **No process management.** No shelling out, no output parsing, no child processes to babysit. Direct function calls with typed return values.
- **Full pipeline access.** The SDK exposes the complete indexing pipeline — scan, chunk, analyze, store, generate. Use the full workflow or compose individual stages.

## Who This Is For

- **Primary:** Platform engineering teams building internal developer tools in Go.
- **Secondary:** Teams building custom CI/CD tooling that needs code understanding.
- **Tertiary:** Companies embedding codebase intelligence into their own products.

## Suggested Messaging

- "Build your own developer tools with codebase intelligence baked in."
- "Three functions. Complete codebase understanding. Import and go."
- "The same engine that powers the Carto CLI, now available as a Go package."
- "Stop shelling out to CLI commands. Embed codebase intelligence directly in your Go applications."
- "Your internal tools deserve the same code understanding that powers Carto."

## Competitive Differentiators

- Not a CLI wrapper — the SDK is the core engine, ensuring feature parity and reliability.
- Minimal API surface (three functions) reduces learning curve and maintenance burden.
- Go-native design with context support enables proper cancellation and timeout handling.
