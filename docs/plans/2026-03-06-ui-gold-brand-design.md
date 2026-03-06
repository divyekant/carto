# UI Gold Brand Overhaul — Design

**Date:** 2026-03-06
**Status:** Approved
**Pencil Reference:** `pencil-new.pen` (Brand Palette + Dashboard Dark/Light mockups)

## Goal

Replace Carto's Indigo/Slate brand with the Memories gold/stone brand, adopting the same Kalos design system tokens for visual consistency across projects. Web UI only — CLI branding stays as-is.

## Color System

### Dark Theme (Default)

| Token | Value | Role |
|-------|-------|------|
| `--color-primary` | `#d4af37` | Primary actions, logo, active nav, focus rings |
| `--color-primary-hover` | `#f5d060` | Hover states |
| `--color-primary-dim` | `rgba(212,175,55,0.12)` | Subtle backgrounds |
| `--color-bg` | `#0a0a0a` | Page background |
| `--color-bg-elevated` | `#111111` | Cards, sidebar |
| `--color-bg-surface` | `#1a1a1a` | Inputs, nested surfaces |
| `--color-text` | `#e5e5e5` | Primary text |
| `--color-text-muted` | `#a3a3a3` | Secondary text |
| `--color-text-faint` | `#666666` | Tertiary text, placeholders |
| `--color-border` | `rgba(212,175,55,0.15)` | Borders |
| `--color-border-subtle` | `rgba(255,255,255,0.06)` | Subtle separators |
| `--color-overlay` | `rgba(0,0,0,0.6)` | Modal overlays |

### Light Theme

| Token | Value | Role |
|-------|-------|------|
| `--color-primary` | `#b8960f` | Primary actions (darkened gold) |
| `--color-primary-hover` | `#d4af37` | Hover states |
| `--color-primary-dim` | `rgba(184,150,15,0.10)` | Subtle backgrounds |
| `--color-bg` | `#FAF9F6` | Page background (warm cream) |
| `--color-bg-elevated` | `#FFFEF9` | Cards, sidebar |
| `--color-bg-surface` | `#F0EDE6` | Inputs, nested surfaces |
| `--color-text` | `#2C2418` | Primary text |
| `--color-text-muted` | `#7A7060` | Secondary text |
| `--color-text-faint` | `#A69E90` | Tertiary text, placeholders |
| `--color-border` | `#E8E2D6` | Borders |
| `--color-border-subtle` | `rgba(0,0,0,0.06)` | Subtle separators |
| `--color-overlay` | `rgba(0,0,0,0.3)` | Modal overlays |

### Semantic Colors (Both Themes)

| Token | Value | Role |
|-------|-------|------|
| `--color-success` | `#16A34A` | Success indicators |
| `--color-warning` | `#CA8A04` | Warnings |
| `--color-error` | `#DC2626` | Errors, destructive actions |
| `--color-info` | `#2563EB` | Info callouts |

## Typography

| Role | Font | Weight | Size |
|------|------|--------|------|
| Display / Page titles | Philosopher | 700 | 28px |
| Section headings | Inter | 600 | 16-18px |
| Body text | Inter | 400 | 14-16px |
| Labels / Metadata | Inter | 500 | 12-13px |
| Code / Paths | System monospace | 400 | 13-14px |

**Loading:** Google Fonts CDN in `index.html`:
```html
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Philosopher:wght@400;700&display=swap" rel="stylesheet">
```

## Spacing

4px base unit (matching Memories Kalos config):

| Token | Value |
|-------|-------|
| `--space-1` | 4px |
| `--space-2` | 8px |
| `--space-3` | 12px |
| `--space-4` | 16px |
| `--space-6` | 24px |
| `--space-8` | 32px |
| `--space-12` | 48px |

## Border Radius

| Token | Value |
|-------|-------|
| `--radius-sm` | 4px |
| `--radius-md` | 8px |
| `--radius-lg` | 12px |
| `--radius-full` | 9999px |

## Transitions

| Token | Value |
|-------|-------|
| `--transition-fast` | 120ms ease |
| `--transition-normal` | 200ms ease |

## Files to Change

| File | Changes |
|------|---------|
| `go/web/index.html` | Add Google Fonts `<link>`, update `theme-color` meta to `#d4af37` |
| `go/web/src/index.css` | Replace all CSS variables — colors, fonts, radius, spacing |
| `go/web/src/components/Layout.tsx` | Sidebar logo mark (gold "C"), nav active states, brand colors |
| `go/web/src/pages/About.tsx` | Update brand palette display with gold colors and roles |
| `go/web/public/favicon.svg` | Gold "C" on dark background |
| `go/web/src/components/ThemeProvider.tsx` | No changes needed (same toggle mechanism) |

## What Stays the Same

- All v1.1.0 layout structure (sidebar, cards, pages)
- shadcn/ui component library (re-themed via CSS variables)
- Theme toggle (light/dark/system)
- All page functionality (Dashboard, Index, Query, Settings, About)
- CLI branding (`branding.go`) — unchanged

## Design Decisions

1. **Gold brand from Memories** — consistent visual identity across projects
2. **Dark theme default** — matches Memories, better for developer tools
3. **Google Fonts CDN** — simpler than self-hosting, acceptable for dev tool
4. **Philosopher for display only** — page titles and brand name; Inter for everything else
5. **4px spacing grid** — disciplined spacing matching Kalos config
6. **Semantic colors unchanged** — green/amber/red/blue are standard, no reason to diverge
