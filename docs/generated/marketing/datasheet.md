---
id: ds-001
type: datasheet
audience: marketing
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Carto Product Datasheet

## Overview

Carto is an intent-aware codebase intelligence platform that gives AI coding assistants deep, structured understanding of your project — automatically. In 90 seconds, Carto scans your codebase, builds a multi-layered semantic index, and delivers that knowledge to tools like Claude and Cursor so they write code that actually fits your architecture, follows your patterns, and respects your conventions.

## Key Capabilities

### Core Intelligence

| Capability | What It Does for You |
|---|---|
| **Automated Codebase Indexing** | Scans and understands your entire codebase in 90 seconds. No manual annotation, no configuration files to maintain. Point it at your repo and go. |
| **Tiered Retrieval** | Delivers exactly the right amount of context for every task — from a quick 5KB summary for small fixes to a comprehensive 500KB deep-dive for architectural decisions. AI assistants get what they need, nothing more. |
| **AI Skill File Generation** | Automatically produces CLAUDE.md and .cursorrules files that plug directly into your AI workflow. Your assistants immediately know your project's patterns, conventions, and architecture. |
| **Multi-Provider LLM Support** | Works with Anthropic, OpenAI, OpenRouter, or fully local models via Ollama. Choose the provider that fits your budget, compliance requirements, or performance needs. |

### Developer Interfaces

| Capability | What It Does for You |
|---|---|
| **CLI** | Nine commands cover the full workflow — scan, index, retrieve, generate. Automate everything from CI pipelines to developer onboarding. |
| **REST API** | Programmatic access with real-time progress streaming via Server-Sent Events. Build codebase intelligence into your own tools and workflows. |
| **Web Dashboard** | A visual interface the whole team can use. Non-technical stakeholders see project structure; developers monitor indexing and retrieve context on demand. |
| **Go SDK** | Embed codebase intelligence directly into your own applications. Build custom integrations without wrestling with HTTP calls. |

### Integrations & Deployment

| Capability | What It Does for You |
|---|---|
| **External Source Integration** | Connects your code to the context that surrounds it — GitHub issues, Jira tickets, Linear projects, Notion docs, and Slack conversations. AI assistants see the full picture, not just the code. |
| **Docker Deployment** | Production-ready in one command. Ship Carto alongside your existing infrastructure with zero friction. |

## Technical Specifications

| Specification | Detail |
|---|---|
| Language | Go 1.25+ |
| Platforms | macOS, Linux, Docker |
| API Protocol | REST + Server-Sent Events (SSE) |
| Authentication | API key, OAuth |
| Storage Backend | Memories (external vector store) |
| LLM Providers | Anthropic, OpenAI, OpenRouter, Ollama |
| Languages Parsed (AST) | Go, TypeScript, JavaScript, Python, Java, Rust |
| Languages Detected | 30+ |
| Deployment Options | Native binary, Docker, Docker Compose |

## Dependencies

| Dependency | Role |
|---|---|
| Tree-sitter | AST-based code parsing for precise structural understanding |
| Cobra | CLI framework powering the command-line interface |
| Memories | Vector storage backend for the semantic index |

## Integrations

- **Memories** — Persistent vector storage for codebase intelligence
- **GitHub** — Pull requests, issues, and repository metadata
- **Jira** — Project tracking and issue context
- **Linear** — Modern issue tracking integration
- **Notion** — Documentation and knowledge base linking
- **Slack** — Team conversation context

## Deployment Options

| Option | Best For |
|---|---|
| **Native Binary** | Individual developers, CI pipelines, scripted workflows |
| **Docker** | Standardized deployment, team environments |
| **Docker Compose** | Full-stack deployment with Memories and all dependencies in one command |

## Security & Compliance

- **API Key Authentication** — Secure access to all endpoints
- **OAuth Support** — Enterprise-grade identity management
- **Local LLM Option** — Run with Ollama for fully air-gapped environments where no code leaves your network
- **No Code Exfiltration** — Carto processes code locally; only semantic summaries are stored

## System Requirements

| Requirement | Detail |
|---|---|
| Go | 1.25+ (for building from source) |
| CGO | Required (Tree-sitter native parsing) |
| Memories Server | Required for index storage and retrieval |
| Docker | 20.10+ (for containerized deployment) |
