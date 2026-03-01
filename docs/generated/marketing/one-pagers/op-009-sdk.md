---
id: op-009
type: one-pager
audience: marketing
topic: Go SDK
status: draft
generated: 2026-02-28
source-tier: carto
hermes-version: 1.0.0
---

# Carto Go SDK: Build Tools With Codebase Intelligence

## The Problem

Your team is building internal developer tools — and those tools need to understand code. But integrating code analysis means shelling out to CLI commands, parsing unstructured output, and managing fragile process chains. It works until it doesn't, and then it's painful to debug.

## The Solution

Carto's Go SDK lets you embed codebase intelligence directly into your applications with three simple functions: Index, Query, and Sources. It's the exact same engine that powers the Carto CLI — battle-tested, well-documented, and ready to import.

## Key Benefits

**Minimal surface area.** Three functions cover the complete workflow. Index builds codebase intelligence, Query retrieves it, Sources manages configurations. Learn it in minutes.

**Production-grade reliability.** This is not a wrapper or a shim. The SDK is the core engine. Every feature in the CLI runs through this exact code. If the CLI works, the SDK works.

**Go-native patterns.** Context support for cancellation and timeouts. Proper error types. Idiomatic interfaces. It integrates with your codebase like any other well-designed Go package.

**No fragile glue.** Direct function calls with typed inputs and outputs. No subprocess management, no output parsing, no string manipulation to extract results.

**Full pipeline control.** Use the high-level functions for common workflows, or access individual pipeline stages when you need fine-grained control over scanning, chunking, analysis, and storage.

## How It Works

Import the Carto package into your Go application. Call `Index` to build codebase intelligence for a project. Call `Query` to retrieve structured context. Call `Sources` to manage which codebases are tracked. Each function returns typed results and standard Go errors.

## Who It's For

Platform engineering teams building internal developer portals. Companies embedding code understanding into their own products. Teams that need codebase intelligence in their Go services without the overhead of HTTP calls or CLI subprocesses.

## Get Started

Add Carto as a Go module dependency, import the SDK package, and make your first `Index` call. The Quick Start guide walks you through it in under five minutes.
