# Per-Project Source Configuration — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a project detail page with per-project source configuration that reads/writes `.carto/sources.yaml`, plus two new API endpoints.

**Architecture:** New `/projects/:name` route with Sources + Index cards. Backend resolves project name → path via `projectsDir`, reads/writes `.carto/sources.yaml` using existing `sources.LoadSourcesConfig` / `sources.ParseSourcesConfig`. Global credentials from `config.Config` are surfaced as boolean availability flags.

**Tech Stack:** React + shadcn/ui (Card, Tabs, Badge, Input, Button, Label), Go net/http handlers, existing `sources` and `config` packages.

---

### Task 1: Backend — GET /api/projects/{name}/sources

**Files:**
- Modify: `go/internal/server/handlers.go`
- Modify: `go/internal/server/routes.go`
- Modify: `go/internal/server/server_test.go`

**Step 1: Write the failing test**

Add to `server_test.go`:

```go
func TestGetProjectSources(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "myproj")
	os.MkdirAll(filepath.Join(projDir, ".carto"), 0o755)

	// Write a sources.yaml
	yaml := []byte("sources:\n  jira:\n    url: https://acme.atlassian.net\n    project: PROJ\n")
	os.WriteFile(filepath.Join(projDir, ".carto", "sources.yaml"), yaml, 0o644)

	cfg := config.Config{GitHubToken: "ghp_test", JiraToken: "jira_test"}
	srv := New(cfg, nil, tmp, nil)

	req := httptest.NewRequest("GET", "/api/projects/myproj/sources", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Should have sources from YAML
	srcs := resp["sources"].(map[string]any)
	jira := srcs["jira"].(map[string]any)
	if jira["project"] != "PROJ" {
		t.Errorf("expected jira project PROJ, got %v", jira["project"])
	}

	// Should have credential availability
	creds := resp["credentials"].(map[string]any)
	if creds["github_token"] != true {
		t.Error("expected github_token true")
	}
	if creds["jira_token"] != true {
		t.Error("expected jira_token true")
	}
	if creds["linear_token"] != false {
		t.Error("expected linear_token false")
	}
}

func TestGetProjectSources_NoYAML(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "myproj")
	os.MkdirAll(projDir, 0o755)

	srv := New(config.Config{}, nil, tmp, nil)

	req := httptest.NewRequest("GET", "/api/projects/myproj/sources", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	srcs := resp["sources"].(map[string]any)
	if len(srcs) != 0 {
		t.Errorf("expected empty sources, got %v", srcs)
	}
}

func TestGetProjectSources_NotFound(t *testing.T) {
	tmp := t.TempDir()
	srv := New(config.Config{}, nil, tmp, nil)

	req := httptest.NewRequest("GET", "/api/projects/nonexistent/sources", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/server/... -run TestGetProjectSources -v`
Expected: FAIL — handler doesn't exist yet.

**Step 3: Implement the handler**

Add to `handlers.go`:

```go
// sourcesResponse is the JSON shape returned by GET /api/projects/{name}/sources.
type sourcesResponse struct {
	Sources     map[string]map[string]string `json:"sources"`
	Credentials map[string]bool             `json:"credentials"`
}

// handleGetSources returns the parsed .carto/sources.yaml for a project
// plus boolean availability of global credentials.
func (s *Server) handleGetSources(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projPath := filepath.Join(s.projectsDir, name)

	if info, err := os.Stat(projPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Parse .carto/sources.yaml (nil if not present).
	yamlCfg, err := sources.LoadSourcesConfig(projPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read sources config: "+err.Error())
		return
	}

	// Build sources map from YAML.
	srcMap := make(map[string]map[string]string)
	if yamlCfg != nil {
		for name, entry := range yamlCfg.Sources {
			srcMap[name] = entry.Settings
		}
	}

	// Build credential availability from current config.
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	creds := map[string]bool{
		"github_token": cfg.GitHubToken != "",
		"jira_token":   cfg.JiraToken != "",
		"jira_email":   cfg.JiraEmail != "",
		"linear_token": cfg.LinearToken != "",
		"notion_token": cfg.NotionToken != "",
		"slack_token":  cfg.SlackToken != "",
	}

	writeJSON(w, http.StatusOK, sourcesResponse{
		Sources:     srcMap,
		Credentials: creds,
	})
}
```

Register in `routes.go`:

```go
s.mux.HandleFunc("GET /api/projects/{name}/sources", s.handleGetSources)
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/server/... -run TestGetProjectSources -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add go/internal/server/handlers.go go/internal/server/routes.go go/internal/server/server_test.go
git commit -m "feat(api): add GET /api/projects/{name}/sources endpoint"
```

---

### Task 2: Backend — PUT /api/projects/{name}/sources

**Files:**
- Modify: `go/internal/server/handlers.go`
- Modify: `go/internal/server/routes.go`
- Modify: `go/internal/server/server_test.go`

**Step 1: Write the failing test**

Add to `server_test.go`:

```go
func TestPutProjectSources(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "myproj")
	os.MkdirAll(projDir, 0o755)

	srv := New(config.Config{}, nil, tmp, nil)

	body := `{"sources":{"jira":{"url":"https://acme.atlassian.net","project":"PROJ"},"linear":{"team":"ENG"}}}`
	req := httptest.NewRequest("PUT", "/api/projects/myproj/sources", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the file was written.
	data, err := os.ReadFile(filepath.Join(projDir, ".carto", "sources.yaml"))
	if err != nil {
		t.Fatalf("sources.yaml not created: %v", err)
	}

	// Parse it back to verify round-trip.
	parsed, err := sources.ParseSourcesConfig(data)
	if err != nil {
		t.Fatalf("failed to parse written YAML: %v", err)
	}
	if _, ok := parsed.Sources["jira"]; !ok {
		t.Error("expected jira in parsed sources")
	}
	if _, ok := parsed.Sources["linear"]; !ok {
		t.Error("expected linear in parsed sources")
	}
}

func TestPutProjectSources_EmptyDeletesFile(t *testing.T) {
	tmp := t.TempDir()
	projDir := filepath.Join(tmp, "myproj")
	os.MkdirAll(filepath.Join(projDir, ".carto"), 0o755)
	os.WriteFile(filepath.Join(projDir, ".carto", "sources.yaml"), []byte("sources:\n  jira:\n    project: X\n"), 0o644)

	srv := New(config.Config{}, nil, tmp, nil)

	body := `{"sources":{}}`
	req := httptest.NewRequest("PUT", "/api/projects/myproj/sources", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// File should be removed.
	if _, err := os.Stat(filepath.Join(projDir, ".carto", "sources.yaml")); !os.IsNotExist(err) {
		t.Error("expected sources.yaml to be deleted for empty sources")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/server/... -run TestPutProjectSources -v`
Expected: FAIL

**Step 3: Implement the handler**

Add to `handlers.go`:

```go
// putSourcesRequest is the JSON body for PUT /api/projects/{name}/sources.
type putSourcesRequest struct {
	Sources map[string]map[string]string `json:"sources"`
}

// handlePutSources writes .carto/sources.yaml for a project.
// An empty sources map deletes the file.
func (s *Server) handlePutSources(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projPath := filepath.Join(s.projectsDir, name)

	if info, err := os.Stat(projPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req putSourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	cartoDir := filepath.Join(projPath, ".carto")
	yamlPath := filepath.Join(cartoDir, "sources.yaml")

	// Empty sources → delete the file.
	if len(req.Sources) == 0 {
		os.Remove(yamlPath)
		writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
		return
	}

	// Build YAML content.
	var buf bytes.Buffer
	buf.WriteString("sources:\n")
	for srcName, settings := range req.Sources {
		buf.WriteString("  " + srcName + ":\n")
		for k, v := range settings {
			buf.WriteString("    " + k + ": " + v + "\n")
		}
	}

	os.MkdirAll(cartoDir, 0o755)
	if err := os.WriteFile(yamlPath, buf.Bytes(), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write sources config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
```

Register in `routes.go`:

```go
s.mux.HandleFunc("PUT /api/projects/{name}/sources", s.handlePutSources)
```

Add `"bytes"` to imports in `handlers.go` if not already present.

**Step 4: Run test to verify it passes**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/server/... -run TestPutProjectSources -v`
Expected: ALL PASS

**Step 5: Run full server tests**

Run: `cd /Users/dk/projects/indexer/go && go test -race ./internal/server/...`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add go/internal/server/handlers.go go/internal/server/routes.go go/internal/server/server_test.go
git commit -m "feat(api): add PUT /api/projects/{name}/sources endpoint"
```

---

### Task 3: Frontend — Project Detail Page (shell + routing)

**Files:**
- Create: `go/web/src/pages/ProjectDetail.tsx`
- Modify: `go/web/src/App.tsx`
- Modify: `go/web/src/components/ProjectCard.tsx`
- Modify: `go/web/src/pages/Dashboard.tsx`

**Step 1: Create ProjectDetail page shell**

Create `go/web/src/pages/ProjectDetail.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

interface Project {
  name: string
  path: string
  indexed_at: string
  file_count: number
}

export default function ProjectDetail() {
  const { name } = useParams<{ name: string }>()
  const navigate = useNavigate()
  const [project, setProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/projects')
      .then(r => r.json())
      .then((data: Project[]) => {
        const projects = Array.isArray(data) ? data : (data as any).projects || []
        const found = projects.find((p: Project) => p.name === name)
        setProject(found || null)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [name])

  if (loading) {
    return (
      <div>
        <h2 className="text-2xl font-bold mb-6">Project</h2>
        <p className="text-muted-foreground">Loading...</p>
      </div>
    )
  }

  if (!project) {
    return (
      <div>
        <h2 className="text-2xl font-bold mb-6">Project Not Found</h2>
        <p className="text-muted-foreground mb-4">No indexed project named "{name}".</p>
        <Button variant="secondary" onClick={() => navigate('/')}>Back to Dashboard</Button>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <button onClick={() => navigate('/')} className="text-muted-foreground hover:text-foreground">
          &larr;
        </button>
        <h2 className="text-2xl font-bold">{project.name}</h2>
        <Badge variant="secondary" className="text-xs">{project.file_count} files</Badge>
      </div>
      <p className="text-sm text-muted-foreground mb-6 truncate" title={project.path}>{project.path}</p>

      <div className="space-y-6 max-w-2xl">
        <Card className="bg-card border-border">
          <CardHeader>
            <CardTitle className="text-base">Sources</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">Source configuration coming next...</p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
```

**Step 2: Add route in App.tsx**

Add import and route:

```tsx
import ProjectDetail from './pages/ProjectDetail'

// Inside <Routes>:
<Route path="/projects/:name" element={<ProjectDetail />} />
```

**Step 3: Make Dashboard cards clickable**

In `Dashboard.tsx`, change the `onReindex` callback to navigate to project detail:

```tsx
// Replace:
onReindex={() => navigate(`/index?path=${encodeURIComponent(p.path)}`)}
// With:
onClick={() => navigate(`/projects/${encodeURIComponent(p.name)}`)}
```

In `ProjectCard.tsx`, replace the `onReindex` prop with `onClick`:

```tsx
// Change interface:
onClick?: () => void  // replaces onReindex

// Change card:
<Card className="bg-card hover:border-primary/30 transition-colors cursor-pointer" onClick={onClick}>

// Remove the Re-index button at the bottom
```

**Step 4: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npx tsc -b && npx vite build`
Expected: Build succeeds.

**Step 5: Commit**

```bash
git add go/web/src/pages/ProjectDetail.tsx go/web/src/App.tsx go/web/src/pages/Dashboard.tsx go/web/src/components/ProjectCard.tsx
git commit -m "feat(ui): add project detail page shell with routing"
```

---

### Task 4: Frontend — Sources Editor Component

**Files:**
- Create: `go/web/src/components/SourcesEditor.tsx`
- Modify: `go/web/src/pages/ProjectDetail.tsx`

**Step 1: Create SourcesEditor component**

Create `go/web/src/components/SourcesEditor.tsx`. This is the core component — it fetches from `GET /api/projects/{name}/sources`, renders toggleable source cards with settings fields, and saves via `PUT`.

```tsx
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

interface SourceDef {
  key: string
  label: string
  credentialKeys: string[]    // which global credentials this source needs
  fields: { key: string; label: string; placeholder: string; required: boolean }[]
}

const SOURCE_DEFS: SourceDef[] = [
  {
    key: 'github',
    label: 'GitHub',
    credentialKeys: ['github_token'],
    fields: [
      { key: 'owner', label: 'Owner', placeholder: 'e.g. divyekant', required: true },
      { key: 'repo', label: 'Repository', placeholder: 'e.g. carto', required: true },
    ],
  },
  {
    key: 'jira',
    label: 'Jira',
    credentialKeys: ['jira_token', 'jira_email'],
    fields: [
      { key: 'url', label: 'Base URL', placeholder: 'https://your-org.atlassian.net', required: true },
      { key: 'project', label: 'Project Key', placeholder: 'e.g. PROJ', required: true },
    ],
  },
  {
    key: 'linear',
    label: 'Linear',
    credentialKeys: ['linear_token'],
    fields: [
      { key: 'team', label: 'Team Key', placeholder: 'e.g. ENG', required: true },
    ],
  },
  {
    key: 'notion',
    label: 'Notion',
    credentialKeys: ['notion_token'],
    fields: [
      { key: 'database', label: 'Database ID', placeholder: 'e.g. abc123-def456', required: true },
    ],
  },
  {
    key: 'slack',
    label: 'Slack',
    credentialKeys: ['slack_token'],
    fields: [
      { key: 'channels', label: 'Channel ID', placeholder: 'e.g. C01234ABC', required: true },
    ],
  },
  {
    key: 'web',
    label: 'Web Pages',
    credentialKeys: [],
    fields: [
      { key: 'urls', label: 'URLs', placeholder: 'https://docs.example.com (comma-separated)', required: true },
    ],
  },
]

interface SourcesEditorProps {
  projectName: string
}

export function SourcesEditor({ projectName }: SourcesEditorProps) {
  const [sources, setSources] = useState<Record<string, Record<string, string>>>({})
  const [credentials, setCredentials] = useState<Record<string, boolean>>({})
  const [enabled, setEnabled] = useState<Record<string, boolean>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetch(`/api/projects/${encodeURIComponent(projectName)}/sources`)
      .then(r => r.json())
      .then(data => {
        setSources(data.sources || {})
        setCredentials(data.credentials || {})
        // Mark sources that have config as enabled.
        const en: Record<string, boolean> = {}
        for (const key of Object.keys(data.sources || {})) {
          en[key] = true
        }
        setEnabled(en)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [projectName])

  function toggleSource(key: string) {
    setEnabled(prev => {
      const next = { ...prev, [key]: !prev[key] }
      if (!next[key]) {
        // Clear settings when disabled.
        setSources(prev => {
          const copy = { ...prev }
          delete copy[key]
          return copy
        })
      }
      return next
    })
  }

  function updateField(sourceKey: string, fieldKey: string, value: string) {
    setSources(prev => ({
      ...prev,
      [sourceKey]: { ...(prev[sourceKey] || {}), [fieldKey]: value },
    }))
  }

  async function save() {
    setSaving(true)
    try {
      // Only send enabled sources.
      const payload: Record<string, Record<string, string>> = {}
      for (const [key, settings] of Object.entries(sources)) {
        if (enabled[key]) {
          payload[key] = settings
        }
      }

      const res = await fetch(`/api/projects/${encodeURIComponent(projectName)}/sources`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sources: payload }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      toast.success('Sources saved')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">Loading sources...</p>
  }

  function credStatus(def: SourceDef): 'ok' | 'missing' | 'na' {
    if (def.credentialKeys.length === 0) return 'na'
    return def.credentialKeys.every(k => credentials[k]) ? 'ok' : 'missing'
  }

  return (
    <div className="space-y-4">
      {SOURCE_DEFS.map(def => {
        const isEnabled = enabled[def.key] || false
        const cred = credStatus(def)
        const settings = sources[def.key] || {}

        return (
          <div key={def.key} className="border border-border rounded-lg p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <button
                  onClick={() => toggleSource(def.key)}
                  className={`w-9 h-5 rounded-full transition-colors relative ${isEnabled ? 'bg-primary' : 'bg-muted'}`}
                >
                  <span className={`block w-3.5 h-3.5 rounded-full bg-white absolute top-0.5 transition-transform ${isEnabled ? 'translate-x-4.5' : 'translate-x-0.5'}`} />
                </button>
                <span className="font-medium text-sm">{def.label}</span>
              </div>
              {cred === 'ok' && <Badge variant="default" className="text-xs">Token configured</Badge>}
              {cred === 'missing' && (
                <a href="/settings" className="text-xs text-amber-500 hover:underline">
                  Set up in Settings &rarr;
                </a>
              )}
            </div>

            {isEnabled && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-2">
                {def.fields.map(field => (
                  <div key={field.key} className="space-y-1">
                    <Label className="text-xs">{field.label}</Label>
                    <Input
                      placeholder={field.placeholder}
                      value={settings[field.key] || ''}
                      onChange={e => updateField(def.key, field.key, e.target.value)}
                    />
                  </div>
                ))}
              </div>
            )}
          </div>
        )
      })}

      <Button onClick={save} disabled={saving}>
        {saving ? 'Saving...' : 'Save Sources'}
      </Button>
    </div>
  )
}
```

**Step 2: Wire into ProjectDetail**

In `ProjectDetail.tsx`, replace the placeholder Sources card:

```tsx
import { SourcesEditor } from '@/components/SourcesEditor'

// Replace the placeholder card with:
<Card className="bg-card border-border">
  <CardHeader>
    <CardTitle className="text-base">Sources</CardTitle>
  </CardHeader>
  <CardContent>
    <SourcesEditor projectName={project.name} />
  </CardContent>
</Card>
```

**Step 3: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npx tsc -b && npx vite build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add go/web/src/components/SourcesEditor.tsx go/web/src/pages/ProjectDetail.tsx
git commit -m "feat(ui): add SourcesEditor component with toggle + settings fields"
```

---

### Task 5: Frontend — Index Card on Project Detail

**Files:**
- Modify: `go/web/src/pages/ProjectDetail.tsx`

**Step 1: Add Index card below Sources**

Add state and SSE handling (reuse pattern from IndexRun.tsx), then add the card:

```tsx
// Add below the Sources card:
<Card className="bg-card border-border">
  <CardHeader>
    <CardTitle className="text-base">Index</CardTitle>
  </CardHeader>
  <CardContent className="space-y-4">
    <div className="flex items-center gap-4">
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={incremental}
          onChange={e => setIncremental(e.target.checked)}
          className="rounded"
        />
        Incremental
      </label>
      <Input
        placeholder="Module filter (optional)"
        value={moduleFilter}
        onChange={e => setModuleFilter(e.target.value)}
        className="max-w-xs"
      />
    </div>
    <Button onClick={startIndex} disabled={indexState === 'running'}>
      {indexState === 'running' ? 'Indexing...' : 'Index Now'}
    </Button>
    {/* Progress and result display — same pattern as IndexRun.tsx */}
  </CardContent>
</Card>
```

The `startIndex` function POSTs to `/api/projects/index` with the project path and connects to SSE for progress.

**Step 2: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npx tsc -b && npx vite build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add go/web/src/pages/ProjectDetail.tsx
git commit -m "feat(ui): add Index card to project detail page"
```

---

### Task 6: Integration Test + Final Verification

**Files:**
- Modify: `go/internal/server/server_test.go` (if needed)

**Step 1: Run full Go test suite**

Run: `cd /Users/dk/projects/indexer/go && go test -race -short ./...`
Expected: ALL PASS

**Step 2: Run go vet**

Run: `cd /Users/dk/projects/indexer/go && go vet ./...`
Expected: Clean

**Step 3: Build Go binary**

Run: `cd /Users/dk/projects/indexer/go && go build -o /dev/null ./cmd/carto`
Expected: Build succeeds

**Step 4: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npx tsc -b && npx vite build`
Expected: Build succeeds

**Step 5: Docker build + deploy**

Run: `cd /Users/dk/projects/indexer/go && docker compose build && docker compose down && docker compose up -d`
Expected: Build succeeds, container starts.

Run: `curl -s http://localhost:8950/api/health | python3 -m json.tool`
Expected: `{"status": "ok", "docker": true, "memories_healthy": true}`

**Step 6: Commit any remaining changes**

```bash
git add -A
git commit -m "test: add integration tests for project sources API"
```
