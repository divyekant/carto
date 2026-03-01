---
id: fb-007
type: feature-brief
audience: marketing
topic: REST API
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: REST API

## One-Liner

Full REST API with real-time progress streaming lets you embed codebase intelligence into any tool or workflow.

## Problem

Teams need codebase understanding woven into their existing systems — CI/CD pipelines, internal developer portals, custom dashboards, and third-party integrations. A CLI alone can't serve a web app. A Go SDK alone can't serve a Python service. Teams need an integration layer that speaks HTTP.

## Solution

Carto exposes every capability through a standard REST API. Index codebases, query context, manage projects, and generate skill files — all via HTTP endpoints. Long-running operations like indexing stream real-time progress updates through Server-Sent Events (SSE), so your integrations always know what's happening.

## Key Benefits

- **Every feature over HTTP.** Anything the CLI can do, the API can do. No features are locked to a specific interface.
- **Real-time progress streaming.** Indexing a large codebase takes time. SSE progress events keep your UI or pipeline informed without polling.
- **Standard REST patterns.** Predictable URLs, JSON payloads, conventional status codes. Your team already knows how to use it.
- **Language-agnostic integration.** Any language that speaks HTTP can integrate with Carto — Python, JavaScript, Ruby, Java, or anything else.
- **Runs alongside the CLI.** Start the server with one command. The API serves the same codebase intelligence your CLI produces.

## Who This Is For

- **Primary:** Platform engineering teams building internal developer tools and portals.
- **Secondary:** CI/CD architects integrating codebase intelligence into automated pipelines.
- **Tertiary:** Teams building custom dashboards or reporting tools that need code context.

## Suggested Messaging

- "Now you can embed codebase intelligence into any tool your team builds."
- "Carto's REST API turns code understanding into a service — accessible from any language, any platform."
- "Real-time progress streaming means your integrations are never in the dark."
- "Standard REST. Standard JSON. Zero learning curve for your engineering team."
- "Build an internal developer portal with codebase intelligence baked in — Carto's API makes it possible."

## Competitive Differentiators

- SSE-based progress streaming eliminates polling and keeps integrations responsive during long operations.
- Full feature parity with the CLI — the API is not a subset.
- Runs as a lightweight embedded server, not a separate infrastructure dependency.
