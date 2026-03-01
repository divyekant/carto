---
id: fb-010
type: feature-brief
audience: marketing
topic: Docker Deployment
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Docker Deployment

## One-Liner

Get Carto running in production with a single `docker compose up` — batteries included.

## Problem

Setting up code analysis infrastructure is time-consuming and complex. You need the right runtime, the right dependencies, a storage backend, and the right configuration — all before you can analyze a single line of code. Most teams spend hours or days getting their code intelligence tools operational.

## Solution

Carto provides a production-ready Docker Compose setup that bundles everything you need. One command brings up the Carto server, its storage backend, and the web dashboard — fully configured and ready to index codebases. No dependency hunting, no configuration wrestling.

## Key Benefits

- **One-command deployment.** `docker compose up` and you're running. No build steps, no dependency resolution, no manual configuration.
- **Batteries included.** The Compose file bundles Carto with all its dependencies. Everything is pre-configured to work together out of the box.
- **Production-ready defaults.** Sensible resource limits, health checks, and restart policies are built in. Deploy with confidence from day one.
- **Runs anywhere Docker runs.** Local development machines, cloud VMs, Kubernetes clusters, internal servers — if it runs Docker, it runs Carto.
- **Consistent environments.** Every team member, every CI runner, every deployment gets the exact same Carto setup. No "works on my machine" surprises.

## Who This Is For

- **Primary:** DevOps and platform engineers deploying Carto for their teams.
- **Secondary:** Engineering leads evaluating Carto who want a fast, reliable setup experience.
- **Tertiary:** Individual developers who want a clean, isolated Carto environment on their local machine.

## Suggested Messaging

- "Production-ready codebase intelligence in one command."
- "Stop spending days on setup. Carto's Docker deployment gets your team running in minutes."
- "Batteries included. Carto, storage, and dashboard — all configured, all running, one command."
- "Deploy Carto anywhere Docker runs. Cloud, on-prem, local — same setup, same results."
- "Your infrastructure team will thank you. One Compose file, zero dependency headaches."

## Competitive Differentiators

- Single-command deployment versus multi-step installation processes required by competing tools.
- All dependencies bundled — no external databases, runtimes, or services to provision separately.
- Same Docker image works for local development and production deployment.
