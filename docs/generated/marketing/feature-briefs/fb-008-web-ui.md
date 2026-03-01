---
id: fb-008
type: feature-brief
audience: marketing
topic: Web Dashboard
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Web Dashboard

## One-Liner

A built-in web dashboard that lets your entire team browse, index, and query codebases without touching the command line.

## Problem

Not everyone on the team uses the CLI. Product managers need to understand how the codebase is structured. Architects need to review cross-component relationships. Engineering leads need visibility into indexing status. New team members need an approachable way to explore unfamiliar code. Today, all of that requires either CLI proficiency or asking someone else.

## Solution

Carto includes a responsive web dashboard embedded directly in the binary. No separate install, no infrastructure to manage. Open a browser, and your entire team can browse indexed codebases, trigger new indexes, run queries, configure project sources, and watch indexing progress in real time.

## Key Benefits

- **Zero setup.** The dashboard is embedded in the Carto binary. Start the server and it's available. No Node.js, no build step, no separate deployment.
- **Live indexing progress.** Watch your codebase intelligence build in real time. Progress indicators show exactly where each indexing run stands.
- **Visual query interface.** Ask questions about your codebase through a clean, intuitive interface. Results are formatted for easy reading and sharing.
- **Per-project source configuration.** Add, remove, and manage codebase sources for each project directly from the dashboard.
- **Responsive design.** Works on desktops, tablets, and laptops. Check indexing status from anywhere.

## Who This Is For

- **Primary:** Engineering leads and architects who need codebase visibility without CLI overhead.
- **Secondary:** Product managers who want to understand system structure and component relationships.
- **Tertiary:** New team members onboarding onto unfamiliar codebases.

## Suggested Messaging

- "Codebase intelligence for the whole team — not just the developers who live in the terminal."
- "Now your architects, PMs, and leads can explore the codebase without asking an engineer."
- "Zero-setup dashboard. Start the server, open a browser, and your whole team has codebase context."
- "Watch your codebase intelligence build in real time. No more wondering if the index is done."
- "One tool, three interfaces. CLI for power users. API for automation. Dashboard for everyone else."

## Competitive Differentiators

- Embedded in the binary — no separate web application to deploy or maintain.
- Not a read-only dashboard. Users can trigger indexes, run queries, and configure sources.
- Same codebase intelligence accessible through CLI, API, and dashboard — complete consistency.
