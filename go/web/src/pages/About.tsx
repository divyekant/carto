// About.tsx — Carto product identity and branding page.
//
// Displays the tagline, product description, who it's for, how it works,
// headline features, and the brand color palette.
// Data is fetched from GET /api/about so it always reflects the server's
// canonical constants (version, features, etc.).

import { useEffect, useState } from 'react'

interface ColorEntry {
  name: string
  hex: string
  role: string
}

interface AboutData {
  name: string
  version: string
  tagline: string
  description: string
  for_whom: string
  how_it_works: string
  features: string[]
  project_url: string
  brand_colors: ColorEntry[]
}

const STEPS = [
  {
    number: '01',
    title: 'Index',
    description:
      'Carto scans your codebase, extracts modules, analyses patterns with LLMs, and stores semantic embeddings in a layered Memories vector store.',
  },
  {
    number: '02',
    title: 'Query',
    description:
      'Ask natural-language questions. Carto retrieves the right code, docs, and context across your entire project history.',
  },
  {
    number: '03',
    title: 'Generate',
    description:
      "Produce CLAUDE.md and .cursorrules files so AI assistants receive a detailed map of your project's intent and architecture.",
  },
  {
    number: '04',
    title: 'Integrate',
    description:
      'Connect GitHub Issues, Jira, Notion, Slack, and PDFs into one unified knowledge graph.',
  },
]

// Fallback data shown while the API call is in-flight or if the fetch fails.
const FALLBACK: AboutData = {
  name: 'Carto',
  version: '—',
  tagline: 'Map your codebase. Navigate with intent.',
  description:
    'Carto indexes your source code, documentation, issues, and knowledge bases into a semantic vector store, making every file, pattern, and architectural decision retrievable by meaning — not just keyword.',
  for_whom:
    'Engineering teams that want AI assistants to understand their whole project. Platform engineers building internal developer portals. CTOs who need codebase-wide insights, automated documentation, and dependency graphs on demand.',
  how_it_works: '',
  features: [
    'Semantic code search across your entire repository',
    'LLM-powered module intent extraction (Anthropic, OpenAI, Ollama)',
    'Layered storage: atoms → modules → blueprints → patterns',
    'CLAUDE.md and .cursorrules generator for AI assistant context',
    'GitHub, Jira, Linear, Notion, Slack, PDF source connectors',
    'Incremental re-indexing — only changed files are re-processed',
    'Docker-native deployment with bearer-auth and audit logging',
  ],
  project_url: 'https://github.com/divyekant/carto',
  brand_colors: [
    { name: 'Gold (dark)', hex: '#d4af37', role: 'Primary actions, logo mark, active states (dark theme)' },
    { name: 'Darkened Gold (light)', hex: '#b8960f', role: 'Primary actions, logo mark, active states (light theme)' },
    { name: 'Success Green', hex: '#16A34A', role: 'Success indicators and healthy status' },
    { name: 'Warning Amber', hex: '#CA8A04', role: 'Warnings and advisory messages' },
    { name: 'Error Red', hex: '#DC2626', role: 'Errors and destructive actions' },
    { name: 'Info Blue', hex: '#2563EB', role: 'Info callouts and informational states' },
  ],
}

export default function About() {
  const [data, setData] = useState<AboutData>(FALLBACK)

  useEffect(() => {
    fetch('/api/about')
      .then((r) => r.json())
      .then((json: AboutData) => setData(json))
      .catch(() => {
        /* keep fallback */
      })
  }, [])

  return (
    <div className="max-w-3xl mx-auto space-y-12 pb-16">
      {/* ── Header ─────────────────────────────────────────────────────── */}
      <section className="space-y-3 pt-2">
        <div className="flex items-baseline gap-3">
          <h1 className="text-4xl font-extrabold tracking-tight text-primary">
            {data.name}
          </h1>
          <span className="text-sm text-muted-foreground font-mono">
            v{data.version}
          </span>
        </div>
        <p className="text-xl font-medium text-foreground/90 leading-snug">
          {data.tagline}
        </p>
        <p className="text-muted-foreground leading-relaxed">{data.description}</p>
      </section>

      {/* ── Who it's for ───────────────────────────────────────────────── */}
      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-foreground">Who is it for?</h2>
        <div className="rounded-lg border border-border bg-card p-5 space-y-2">
          {[
            'Engineering teams that want AI assistants to understand their whole project, not just the file currently open.',
            'Platform engineers building internal developer portals.',
            'CTOs who need codebase-wide insights, automated documentation, and dependency graphs on demand.',
          ].map((line) => (
            <div key={line} className="flex items-start gap-3">
              <span className="mt-1 shrink-0 w-1.5 h-1.5 rounded-full bg-primary" />
              <p className="text-sm text-muted-foreground leading-relaxed">{line}</p>
            </div>
          ))}
        </div>
      </section>

      {/* ── How it works ───────────────────────────────────────────────── */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">How it works</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          {STEPS.map((step) => (
            <div
              key={step.number}
              className="rounded-lg border border-border bg-card p-5 space-y-2"
            >
              <div className="flex items-center gap-3">
                <span className="text-xs font-mono font-bold text-primary/60 tabular-nums">
                  {step.number}
                </span>
                <h3 className="font-semibold text-foreground">{step.title}</h3>
              </div>
              <p className="text-sm text-muted-foreground leading-relaxed">
                {step.description}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ── Features ───────────────────────────────────────────────────── */}
      <section className="space-y-3">
        <h2 className="text-lg font-semibold text-foreground">Headline features</h2>
        <ul className="space-y-2">
          {data.features.map((f) => (
            <li key={f} className="flex items-start gap-3">
              <span className="mt-1 shrink-0 w-1.5 h-1.5 rounded-full bg-primary" />
              <span className="text-sm text-muted-foreground">{f}</span>
            </li>
          ))}
        </ul>
      </section>

      {/* ── Brand colors ───────────────────────────────────────────────── */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">Brand color palette</h2>
        <div className="space-y-2">
          {data.brand_colors.map((color) => (
            <div
              key={color.hex}
              className="flex items-center gap-4 rounded-md border border-border bg-card px-4 py-3"
            >
              {/* Color swatch */}
              <div
                className="shrink-0 w-8 h-8 rounded-md border border-black/10 dark:border-white/10"
                style={{ backgroundColor: color.hex }}
                aria-label={color.name}
              />
              <div className="min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium text-foreground">
                    {color.name}
                  </span>
                  <code className="text-xs font-mono text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                    {color.hex}
                  </code>
                </div>
                <p className="text-xs text-muted-foreground mt-0.5">{color.role}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Typography note */}
        <p className="text-xs text-muted-foreground">
          The light theme uses a warm stone neutral base with darkened gold (#b8960f)
          as the interactive accent. The dark theme uses a near-black base with
          gold (#d4af37). Typography uses Philosopher for display headings and
          Inter for body text. Semantic state colors (success, warning, error, info)
          follow the palette above.
        </p>
      </section>

      {/* ── Project link ───────────────────────────────────────────────── */}
      <section>
        <a
          href={data.project_url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 text-sm text-primary hover:underline font-medium"
        >
          {data.project_url}
          <svg
            width="12"
            height="12"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            aria-hidden="true"
          >
            <path d="M2.5 9.5L9.5 2.5M9.5 2.5H5M9.5 2.5V7" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </a>
      </section>
    </div>
  )
}
