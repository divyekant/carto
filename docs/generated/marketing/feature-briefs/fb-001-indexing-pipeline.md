---
id: fb-001
type: feature-brief
audience: marketing
topic: Indexing Pipeline
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Feature Brief: Indexing Pipeline

## One-Liner

Carto automatically builds a complete semantic understanding of your codebase in under 90 seconds.

## What It Is

An automated code analysis engine that understands not just what your code does, but why it exists. Carto reads your entire codebase, identifies architecture, patterns, conventions, and domain logic, then packages that understanding so AI coding assistants can use it immediately.

This is not keyword search. This is not a file index. Carto builds a layered semantic model of your codebase -- the same kind of understanding a senior engineer develops after months on a project.

## Who It's For

**Primary:** Engineering teams using AI coding assistants (Claude Code, Cursor, GitHub Copilot). These teams want AI-generated code that actually fits their project -- not generic boilerplate.

**Secondary:** Engineering leaders who want consistent, high-quality AI-assisted development across their organization. One indexing run means every developer's AI assistant understands the same architecture and follows the same patterns.

## The Problem

Every time an AI coding assistant opens your project, it starts from zero. It doesn't know your architecture. It doesn't know your naming conventions. It doesn't know why that service layer exists or what patterns your team agreed on last quarter.

The result: generic code that technically works but doesn't belong in your codebase. Developers spend more time correcting AI output than they save using it.

## Key Benefits

- **Instant context, no manual docs.** Carto generates understanding automatically. No one has to write or maintain documentation for AI assistants.
- **Incremental updates.** Only re-indexes files that changed. After the first run, updates take seconds, not minutes.
- **Language-agnostic.** Production-grade AST parsing for Go, TypeScript, Python, Java, Rust, and 25+ additional languages detected automatically.
- **Deep, not shallow.** Carto uses AI to analyze cross-component relationships, not just individual files. It understands how your modules connect and why.

## How It Works (Simplified)

Three steps, fully automated:

1. **Scan** -- Carto discovers your code, respects your .gitignore, and identifies project structure.
2. **Analyze** -- AI models extract meaning at multiple levels: individual functions, module relationships, and system-wide architecture.
3. **Store** -- The understanding is stored for instant retrieval, ready for any AI assistant to use.

The entire process runs in under 90 seconds for a typical project. No configuration required.

## Competitive Context

No competing tool offers layered semantic indexing with tiered retrieval specifically designed for AI coding assistants. Existing solutions either provide shallow file-level search or require extensive manual setup. Carto is the only product that delivers deep architectural understanding automatically and makes it immediately usable by AI assistants.

## Suggested Messaging

**Announcement:** "Carto now indexes your entire codebase in under 90 seconds -- giving every AI coding assistant on your team instant, deep understanding of your architecture, patterns, and conventions."

**Sales Pitch:** "Your AI assistants are writing code blind. Carto gives them the same understanding of your codebase that your senior engineers have -- automatically, in 90 seconds, and kept up to date with every change."

**One-Liner:** "90 seconds to make every AI assistant an expert in your codebase."
