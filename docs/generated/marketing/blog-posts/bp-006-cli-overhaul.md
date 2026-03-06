---
id: bp-006
type: blog-post
audience: marketing
topic: CLI & Agent Usability
status: draft
generated: 2026-03-06
source-tier: carto
hermes-version: 1.0.0
---

# Making Carto Agent-Native: How We Built a CLI That Speaks Human and Machine

Something shifted in how developer tools get used. A year ago, the typical Carto user was a developer typing commands in their terminal. Today, more than half of CLI invocations come from AI agents -- Claude, Cursor, Copilot -- running commands on behalf of developers. And those agents were struggling.

They were parsing colored text. They were waiting out progress spinners. They were guessing whether an error was a warning or a failure based on the wording of a sentence meant for human eyes. It worked, mostly. But "mostly" is not good enough when your AI assistant is making decisions based on CLI output.

We decided to fix this properly.

## The Problem: CLIs Were Not Built for AI Agents

Every CLI you have ever used was designed with one consumer in mind: a person reading text in a terminal window. Colors convey meaning. Spinners convey progress. Error messages are written in natural language because humans read natural language.

AI agents are not that consumer. They need structure. They need predictable envelopes. They need typed error codes they can branch on, not sentences they have to interpret. And they need all of this without the developer having to remember a `--json` flag or wrap every command in a parser.

The industry's answer so far has been to bolt on machine-readable output as an afterthought -- add a `--json` flag, hope someone uses it, and call it a day. That puts the burden on the agent (or the developer configuring the agent) to know that the flag exists and to always use it. It is fragile, easy to forget, and fundamentally backwards.

## Our Approach: Detect the Consumer, Adapt the Output

We started from a simple observation: you can tell who is calling your CLI. If the output is connected to a terminal (a TTY), a human is watching. If it is piped, redirected, or called from a subprocess, a machine is consuming it.

This is not a new technique -- Unix tools have done TTY detection for decades. What is new is building an entire CLI experience around it as a first-class design principle rather than a cosmetic toggle.

When Carto detects a TTY, it delivers the full human experience: gold-branded colored output, progress spinners, interactive prompts, and formatted tables. When it detects a non-TTY context, it switches to a structured JSON envelope with typed fields for status, data, errors, and metadata. Same command. Same arguments. Two completely different output experiences, each optimized for its consumer.

## How It Works: The Envelope Contract

Every Carto command, in machine mode, returns a JSON envelope with a predictable shape:

- A **status** field that is always "ok" or "error" -- no ambiguity.
- A **data** field containing the command's output as structured objects.
- An **error** field with a typed error code and a human-readable message when something goes wrong.
- A **metadata** field with timing, version, and context information.

AI agents can parse this envelope once and handle every Carto command. They can branch on error codes instead of regex-matching error messages. They can extract structured data without screen-scraping. They can detect success or failure with a single field check.

For humans, none of this is visible. The terminal experience is purely visual -- colors, alignment, spinners -- because that is what humans process best.

Confirmation guards add another layer of intelligence. Destructive operations prompt for confirmation when a human is at the keyboard. When an agent is driving, those prompts are automatically bypassed because agents express intent through the command itself, not through interactive dialogs.

## What We Added: Six New Commands

The output overhaul was the foundation, but we also expanded what the CLI can do.

**`carto init`** replaces manual configuration with an interactive setup wizard. It detects your project structure, suggests defaults, and writes configuration in seconds. For agents, it accepts all parameters as flags for fully automated setup.

**`carto completions`** generates shell autocompletion for bash, zsh, fish, and PowerShell. Small quality-of-life improvement, big reduction in friction.

**`carto export` and `carto import`** make your codebase intelligence portable. Export your entire index as NDJSON, move it to another machine, share it with a teammate, or store it as a backup. Your data is not locked in.

**`carto logs`** exposes the structured audit trail. Every command Carto runs is logged with timestamps, parameters, outcomes, and durations. Query the log to understand what happened, when, and why.

**`carto upgrade`** checks for the latest version and updates the binary in place. No package managers. No manual downloads. One command, always current.

## What We Learned

Building for two consumers simultaneously forced better design everywhere. The structured envelope made us think harder about what each command actually returns. The TTY detection made us reconsider which information is essential (always present) versus presentational (only in human mode). The typed error codes made us catalog every failure mode instead of relying on ad-hoc error strings.

The result is a CLI that is better for both audiences -- not a compromise, but a genuine improvement for each.

## What Is Next

The agent-native foundation opens new possibilities. We are exploring watch mode for continuous re-indexing, team sync for shared intelligence across organizations, and a plugin system that lets teams extend Carto's CLI with custom commands. Each of these features will be agent-native from day one.

The era of CLIs built exclusively for human eyes is ending. The tools developers use are increasingly driven by AI agents acting on their behalf. Building for that reality -- not bolting it on as an afterthought -- is what makes a tool truly ready for the AI-assisted development workflow.

Carto v1.2.0 is available now. Your AI agents will notice the difference immediately.
