---
id: op-010
type: one-pager
audience: marketing
topic: Docker Deployment
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Carto Docker Deployment: Production-Ready in One Command

## The Problem

Getting code analysis tools into production is a project in itself. Install the right runtime, provision a database, configure networking, tune resource limits, set up health checks — and hope nothing breaks when a new team member tries to replicate the setup. Most teams spend hours or days before they analyze their first codebase.

## The Solution

Carto ships with a Docker Compose configuration that bundles everything into a single, reproducible deployment. Run `docker compose up` and you have a fully operational Carto instance — server, storage backend, and web dashboard — ready to index codebases.

## Key Benefits

**Minutes, not days.** One command brings up a complete, working Carto deployment. No dependency hunting, no configuration files to write from scratch.

**Everything included.** The Compose setup bundles Carto with its storage backend and dashboard. All components are pre-configured to work together.

**Production-grade from the start.** Health checks, restart policies, and sensible resource defaults are built in. You're not deploying a demo — you're deploying the real thing.

**Runs anywhere.** Local machines, cloud VMs, on-premises servers, Kubernetes clusters — if Docker runs there, Carto runs there. Same image, same behavior.

**Reproducible environments.** Every developer, every CI runner, every staging environment gets an identical Carto setup. Configuration drift is eliminated by design.

## How It Works

Clone the repository or download the Compose file. Set your API key in the environment. Run `docker compose up`. Carto starts, initializes its storage, and serves the web dashboard. Point it at your codebase and start indexing.

## Who It's For

DevOps and platform engineers deploying Carto for development teams. Engineering leads running a quick evaluation. Individual developers who want a clean, isolated setup without touching their system environment.

## Get Started

Download the Docker Compose file, set your API key, and run `docker compose up`. Your Carto instance will be ready to index in under a minute. Visit the deployment guide for full details.
