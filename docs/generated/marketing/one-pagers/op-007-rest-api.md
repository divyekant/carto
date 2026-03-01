---
id: op-007
type: one-pager
audience: marketing
topic: REST API
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Carto REST API: Codebase Intelligence as a Service

## The Problem

Your team's tools need codebase context — internal portals, CI pipelines, custom dashboards, onboarding systems. But building code understanding into each one means reinventing the wheel every time. You need a single, reliable source of codebase intelligence that any system can talk to.

## The Solution

Carto provides a full REST API that exposes every codebase intelligence feature over HTTP. Index projects, query context, manage sources, and generate skill files — all through standard JSON endpoints. Long-running operations stream real-time progress updates so your integrations stay informed.

## Key Benefits

**Complete feature access.** Every capability Carto offers is available through the API. Nothing is locked behind the CLI or the dashboard.

**Real-time visibility.** Indexing a large codebase? SSE progress streaming gives your tools live updates — no polling loops, no guessing.

**Works with any language.** Python, JavaScript, Java, Ruby — if it speaks HTTP, it can use Carto. No SDK required (though the Go SDK is there if you want it).

**Standard patterns.** Predictable URLs, JSON request and response bodies, conventional HTTP status codes. Your developers will feel at home immediately.

**Lightweight deployment.** The API runs as an embedded server inside the same Carto binary. No separate service to deploy or manage.

## How It Works

Start the Carto server with a single command. It exposes REST endpoints for every operation — indexing, querying, project management, and skill file generation. Point your internal tools at the API, authenticate with an API key, and start pulling codebase intelligence into your workflows.

## Who It's For

Platform engineering teams building internal developer portals. CI/CD architects who need codebase context in their pipelines. Any team building custom tools that benefit from structured code understanding.

## Get Started

Launch the Carto server, explore the API endpoints, and make your first query in minutes. Check the API documentation to see what's possible.
