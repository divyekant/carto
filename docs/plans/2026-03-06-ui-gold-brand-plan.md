# UI Gold Brand Overhaul — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace Carto's Indigo/Slate brand with the Memories gold/stone brand across all web UI files.

**Architecture:** Pure CSS variable swap + font addition. No layout changes. The shadcn/ui component library is re-themed via CSS custom properties in `index.css`. Server-side brand palette in `handlers.go` is updated to match. Favicon gets a gold mark.

**Tech Stack:** React + Tailwind CSS 4 + shadcn/ui (frontend), Go net/http (backend API)

---

## Task 1: Add Google Fonts and update meta theme-color

**Files:**
- Modify: `go/web/index.html:1-15`

**Step 1: Edit index.html**

Replace the entire file content with:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="description" content="Carto — Map your codebase. Navigate with intent. Intent-aware codebase intelligence for engineering teams." />
    <meta name="theme-color" content="#d4af37" />
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Philosopher:wght@400;700&display=swap" rel="stylesheet" />
    <title>Carto — Map your codebase. Navigate with intent.</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

Changes:
- Line 8: `theme-color` changed from `#5B50F5` to `#d4af37`
- Lines 9-11: Added Google Fonts preconnect + Inter + Philosopher

**Step 2: Verify it loads**

Run: `cd go/web && npm run dev`
Open browser — confirm Inter and Philosopher fonts load in the Network tab.

**Step 3: Commit**

```bash
git add go/web/index.html
git commit -m "feat(web): add Inter + Philosopher fonts, update theme-color to gold"
```

---

## Task 2: Replace CSS variables with gold/stone palette

**Files:**
- Modify: `go/web/src/index.css:1-130`

**Step 1: Replace entire index.css**

```css
@import "tailwindcss";
@import "tw-animate-css";
@import "shadcn/tailwind.css";

@custom-variant dark (&:is(.dark *));

@theme inline {
    --radius-sm: 4px;
    --radius-md: 8px;
    --radius-lg: 12px;
    --radius-xl: 16px;
    --radius-2xl: 20px;
    --radius-3xl: 24px;
    --radius-4xl: 28px;
    --color-background: var(--background);
    --color-foreground: var(--foreground);
    --color-card: var(--card);
    --color-card-foreground: var(--card-foreground);
    --color-popover: var(--popover);
    --color-popover-foreground: var(--popover-foreground);
    --color-primary: var(--primary);
    --color-primary-foreground: var(--primary-foreground);
    --color-secondary: var(--secondary);
    --color-secondary-foreground: var(--secondary-foreground);
    --color-muted: var(--muted);
    --color-muted-foreground: var(--muted-foreground);
    --color-accent: var(--accent);
    --color-accent-foreground: var(--accent-foreground);
    --color-destructive: var(--destructive);
    --color-border: var(--border);
    --color-input: var(--input);
    --color-ring: var(--ring);
    --color-chart-1: var(--chart-1);
    --color-chart-2: var(--chart-2);
    --color-chart-3: var(--chart-3);
    --color-chart-4: var(--chart-4);
    --color-chart-5: var(--chart-5);
    --color-sidebar: var(--sidebar);
    --color-sidebar-foreground: var(--sidebar-foreground);
    --color-sidebar-primary: var(--sidebar-primary);
    --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);
    --color-sidebar-accent: var(--sidebar-accent);
    --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
    --color-sidebar-border: var(--sidebar-border);
    --color-sidebar-ring: var(--sidebar-ring);
    --font-sans: 'Inter', system-ui, -apple-system, sans-serif;
    --font-display: 'Philosopher', serif;
}

/* ============================================================
   Light theme — stone neutral base, darkened gold (#b8960f) accent
   ============================================================ */
:root {
    --radius: 12px;
    --background: #FAF9F6;
    --foreground: #2C2418;
    --card: #FFFEF9;
    --card-foreground: #2C2418;
    --popover: #FFFEF9;
    --popover-foreground: #2C2418;
    --primary: #b8960f;
    --primary-foreground: #FFFFFF;
    --secondary: #F0EDE6;
    --secondary-foreground: #2C2418;
    --muted: #F0EDE6;
    --muted-foreground: #7A7060;
    --accent: rgba(184, 150, 15, 0.10);
    --accent-foreground: #b8960f;
    --destructive: #DC2626;
    --border: #E8E2D6;
    --input: #E8E2D6;
    --ring: #b8960f;
    --chart-1: #b8960f;
    --chart-2: #16A34A;
    --chart-3: #CA8A04;
    --chart-4: #2563EB;
    --chart-5: #DC2626;
    --sidebar: #FFFEF9;
    --sidebar-foreground: #2C2418;
    --sidebar-primary: #b8960f;
    --sidebar-primary-foreground: #FFFFFF;
    --sidebar-accent: rgba(184, 150, 15, 0.10);
    --sidebar-accent-foreground: #b8960f;
    --sidebar-border: #E8E2D6;
    --sidebar-ring: #b8960f;
}

/* ============================================================
   Dark theme — near-black base, gold (#d4af37) accent
   ============================================================ */
.dark {
    --background: #0a0a0a;
    --foreground: #e5e5e5;
    --card: #111111;
    --card-foreground: #e5e5e5;
    --popover: #111111;
    --popover-foreground: #e5e5e5;
    --primary: #d4af37;
    --primary-foreground: #0a0a0a;
    --secondary: #1a1a1a;
    --secondary-foreground: #e5e5e5;
    --muted: #1a1a1a;
    --muted-foreground: #a3a3a3;
    --accent: rgba(212, 175, 55, 0.12);
    --accent-foreground: #d4af37;
    --destructive: #DC2626;
    --border: rgba(212, 175, 55, 0.15);
    --input: rgba(255, 255, 255, 0.06);
    --ring: #d4af37;
    --chart-1: #d4af37;
    --chart-2: #16A34A;
    --chart-3: #CA8A04;
    --chart-4: #2563EB;
    --chart-5: #DC2626;
    --sidebar: #111111;
    --sidebar-foreground: #e5e5e5;
    --sidebar-primary: #d4af37;
    --sidebar-primary-foreground: #0a0a0a;
    --sidebar-accent: rgba(212, 175, 55, 0.12);
    --sidebar-accent-foreground: #d4af37;
    --sidebar-border: rgba(212, 175, 55, 0.15);
    --sidebar-ring: #d4af37;
}

@layer base {
  * {
    @apply border-border outline-ring/50;
    }
  body {
    @apply bg-background text-foreground;
    font-family: var(--font-sans);
    }
}
```

Key changes:
- Radius values: fixed px values (4/8/12) instead of calc-based
- Added `--font-sans` and `--font-display` theme variables
- Light theme: stone neutrals (`#FAF9F6`, `#FFFEF9`, `#F0EDE6`, `#E8E2D6`) with `#b8960f` primary
- Dark theme: near-black (`#0a0a0a`, `#111111`, `#1a1a1a`) with `#d4af37` primary
- Semantic chart colors: gold, green, amber, blue, red
- Body font-family set via `var(--font-sans)`

**Step 2: Verify both themes**

Run: `cd go/web && npm run dev`
- Toggle dark/light — confirm gold accent, warm backgrounds, no broken colors.

**Step 3: Commit**

```bash
git add go/web/src/index.css
git commit -m "feat(web): replace indigo/slate CSS variables with gold/stone palette"
```

---

## Task 3: Update favicon

**Files:**
- Modify: `go/web/public/favicon.svg:1-4`

**Step 1: Replace favicon.svg**

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#0a0a0a"/>
  <text x="16" y="23" text-anchor="middle" font-family="system-ui,-apple-system,sans-serif" font-weight="700" font-size="20" fill="#d4af37">C</text>
</svg>
```

Changes:
- Background: `#1e293b` (slate) → `#0a0a0a` (near-black)
- Letter color: `#f1f5f9` (slate-100) → `#d4af37` (gold)

**Step 2: Verify**

Hard refresh browser — confirm gold "C" on dark background in the tab.

**Step 3: Commit**

```bash
git add go/web/public/favicon.svg
git commit -m "style: update favicon to gold C on dark background"
```

---

## Task 4: Update sidebar branding in Layout.tsx

**Files:**
- Modify: `go/web/src/components/Layout.tsx:47-49,152`

**Step 1: Update the logo section**

In `Layout.tsx`, replace lines 47-50 (the Logo section inside `SidebarContent`):

```tsx
      {/* Logo */}
      <div className="px-4 py-3 border-b border-border h-12 flex items-center gap-2">
        <span className="inline-flex items-center justify-center w-7 h-7 rounded-md bg-primary text-primary-foreground text-sm font-bold font-[var(--font-display)]">C</span>
        <span className="text-lg font-bold font-[var(--font-display)]">arto</span>
      </div>
```

Changes:
- Logo "C" is now a gold-filled rounded square (using `bg-primary text-primary-foreground`)
- Both "C" and "arto" use `font-[var(--font-display)]` (Philosopher)
- Added `gap-2` for spacing between mark and text

**Step 2: Update mobile header brand**

In `Layout.tsx`, replace line 152:

From:
```tsx
        <span className="text-sm font-bold tracking-tight text-primary">Carto</span>
```

To:
```tsx
        <span className="text-sm font-bold tracking-tight text-primary font-[var(--font-display)]">Carto</span>
```

**Step 3: Verify**

Check both desktop sidebar and mobile header — gold logo mark, Philosopher font for brand name.

**Step 4: Commit**

```bash
git add go/web/src/components/Layout.tsx
git commit -m "feat(web): update sidebar logo to gold mark with Philosopher font"
```

---

## Task 5: Update About page brand colors

**Files:**
- Modify: `go/web/src/pages/About.tsx:75-81,197-203`

**Step 1: Update fallback brand_colors array**

In `About.tsx`, replace lines 75-81 (the `brand_colors` array in `FALLBACK`):

```tsx
  brand_colors: [
    { name: 'Gold (dark)', hex: '#d4af37', role: 'Primary actions, logo mark, active states (dark theme)' },
    { name: 'Darkened Gold (light)', hex: '#b8960f', role: 'Primary actions, logo mark, active states (light theme)' },
    { name: 'Success Green', hex: '#16A34A', role: 'Success indicators and healthy status' },
    { name: 'Warning Amber', hex: '#CA8A04', role: 'Warnings and advisory messages' },
    { name: 'Error Red', hex: '#DC2626', role: 'Errors and destructive actions' },
    { name: 'Info Blue', hex: '#2563EB', role: 'Info callouts and informational states' },
  ],
```

**Step 2: Update typography note**

In `About.tsx`, replace lines 197-203 (the typography note paragraph):

```tsx
        <p className="text-xs text-muted-foreground">
          The light theme uses a warm stone neutral base with darkened gold (#b8960f)
          as the interactive accent. The dark theme uses a near-black base with
          gold (#d4af37). Typography uses Philosopher for display headings and
          Inter for body text. Semantic state colors (success, warning, error, info)
          follow the palette above.
        </p>
```

**Step 3: Verify**

Open `/about` — confirm 6 color swatches with gold primaries and updated semantic colors.

**Step 4: Commit**

```bash
git add go/web/src/pages/About.tsx
git commit -m "feat(web): update About page brand palette to gold + semantic colors"
```

---

## Task 6: Update server-side brand palette

**Files:**
- Modify: `go/internal/server/handlers.go:1049-1055`

**Step 1: Update palette in handleAbout**

In `handlers.go`, replace lines 1049-1055 (the `palette` slice):

```go
	palette := []paletteEntry{
		{"Gold (dark)", "#d4af37", "Primary actions, logo mark, active states (dark theme)"},
		{"Darkened Gold (light)", "#b8960f", "Primary actions, logo mark, active states (light theme)"},
		{"Success Green", "#16A34A", "Success indicators and healthy status"},
		{"Warning Amber", "#CA8A04", "Warnings and advisory messages"},
		{"Error Red", "#DC2626", "Errors and destructive actions"},
		{"Info Blue", "#2563EB", "Info callouts and informational states"},
	}
```

**Step 2: Run tests**

Run: `cd go && go test ./internal/server/ -v -run TestAbout`
If no test for about exists: `cd go && go build ./...` to verify compilation.

**Step 3: Commit**

```bash
git add go/internal/server/handlers.go
git commit -m "feat(api): update /api/about brand palette to gold + semantic colors"
```

---

## Task 7: Full build verification

**Step 1: Build Go binary**

Run: `cd go && go build -o carto ./cmd/carto`
Expected: Compiles with no errors.

**Step 2: Build frontend**

Run: `cd go/web && npm run build`
Expected: Builds with no errors.

**Step 3: Run all tests**

Run: `cd go && go test -short ./...`
Expected: All tests pass.

**Step 4: Visual verification**

Run: `cd go && ./carto serve --port 8950`
Open `http://localhost:8950` — check:
- [ ] Dark theme: gold sidebar, gold buttons, near-black backgrounds
- [ ] Light theme: darkened gold sidebar, cream/stone backgrounds
- [ ] Favicon shows gold "C"
- [ ] About page shows 6 gold/semantic colors
- [ ] All pages readable, no broken styles

**Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix(web): visual polish from gold brand verification"
```
