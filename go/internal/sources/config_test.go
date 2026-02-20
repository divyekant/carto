package sources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSourcesConfig(t *testing.T) {
	yamlData := `
sources:
  jira:
    url: https://mycompany.atlassian.net
    project: PROJ
  slack:
    channels: "#engineering,#architecture"
  web:
    urls: "https://docs.example.com/api,https://docs.example.com/guide"
  github:
    owner: myorg
    repo: myrepo
    discussions: true
`
	cfg, err := ParseSourcesConfig([]byte(yamlData))
	if err != nil {
		t.Fatalf("ParseSourcesConfig: %v", err)
	}

	// Verify jira settings parsed.
	jira, ok := cfg.Sources["jira"]
	if !ok {
		t.Fatal("jira source not found")
	}
	if jira.Settings["project"] != "PROJ" {
		t.Errorf("jira project: got %q, want %q", jira.Settings["project"], "PROJ")
	}
	if jira.Settings["url"] != "https://mycompany.atlassian.net" {
		t.Errorf("jira url: got %q", jira.Settings["url"])
	}

	// Verify slack settings.
	slack, ok := cfg.Sources["slack"]
	if !ok {
		t.Fatal("slack source not found")
	}
	if slack.Settings["channels"] != "#engineering,#architecture" {
		t.Errorf("slack channels: got %q", slack.Settings["channels"])
	}

	// Verify web settings.
	web, ok := cfg.Sources["web"]
	if !ok {
		t.Fatal("web source not found")
	}
	if web.Settings["urls"] == "" {
		t.Error("web urls should not be empty")
	}

	// Verify github settings.
	gh, ok := cfg.Sources["github"]
	if !ok {
		t.Fatal("github source not found")
	}
	if gh.Settings["owner"] != "myorg" {
		t.Errorf("github owner: got %q", gh.Settings["owner"])
	}
	if gh.Settings["discussions"] != "true" {
		t.Errorf("github discussions: got %q", gh.Settings["discussions"])
	}
}

func TestParseSourcesConfig_ListSettings(t *testing.T) {
	yamlData := `
sources:
  slack:
    channels:
      - "#engineering"
      - "#architecture"
  web:
    urls:
      - https://docs.example.com/api
      - https://docs.example.com/guide
`
	cfg, err := ParseSourcesConfig([]byte(yamlData))
	if err != nil {
		t.Fatalf("ParseSourcesConfig: %v", err)
	}

	slack := cfg.Sources["slack"]
	if len(slack.ListSettings["channels"]) != 2 {
		t.Errorf("slack channels: got %d items, want 2", len(slack.ListSettings["channels"]))
	}

	web := cfg.Sources["web"]
	if len(web.ListSettings["urls"]) != 2 {
		t.Errorf("web urls: got %d items, want 2", len(web.ListSettings["urls"]))
	}
}

func TestParseSourcesConfig_Empty(t *testing.T) {
	cfg, err := ParseSourcesConfig([]byte(""))
	if err != nil {
		t.Fatalf("ParseSourcesConfig: %v", err)
	}
	if len(cfg.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(cfg.Sources))
	}
}

func TestParseSourcesConfig_InvalidYAML(t *testing.T) {
	_, err := ParseSourcesConfig([]byte("{{invalid yaml"))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadSourcesConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadSourcesConfig(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestLoadSourcesConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	cartoDir := filepath.Join(dir, ".carto")
	os.MkdirAll(cartoDir, 0o755)

	yamlContent := `
sources:
  jira:
    project: TEST
    url: https://test.atlassian.net
`
	os.WriteFile(filepath.Join(cartoDir, "sources.yaml"), []byte(yamlContent), 0o644)

	cfg, err := LoadSourcesConfig(dir)
	if err != nil {
		t.Fatalf("LoadSourcesConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Sources["jira"].Settings["project"] != "TEST" {
		t.Error("jira project not parsed from file")
	}
}

func TestSaveSourcesConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &SourcesYAML{
		Sources: map[string]SourceEntry{
			"github": {Settings: map[string]string{"owner": "test", "repo": "app"}},
			"jira":   {Settings: map[string]string{"project": "PROJ"}},
		},
	}
	if err := SaveSourcesConfig(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadSourcesConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Sources["github"].Settings["owner"] != "test" {
		t.Fatalf("expected owner=test, got %s", loaded.Sources["github"].Settings["owner"])
	}
	if loaded.Sources["jira"].Settings["project"] != "PROJ" {
		t.Fatalf("expected project=PROJ")
	}
}

func TestSaveSourcesConfig_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "project")
	// .carto dir doesn't exist yet — Save should create it.
	cfg := &SourcesYAML{
		Sources: map[string]SourceEntry{
			"web": {Settings: map[string]string{"urls": "https://example.com"}},
		},
	}
	if err := SaveSourcesConfig(nested, cfg); err != nil {
		t.Fatalf("save to new dir: %v", err)
	}
	loaded, err := LoadSourcesConfig(nested)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Sources["web"].Settings["urls"] != "https://example.com" {
		t.Fatalf("expected urls=https://example.com, got %s", loaded.Sources["web"].Settings["urls"])
	}
}

func TestSaveSourcesConfig_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	// Make the dir read-only so MkdirAll for .carto will fail.
	os.Chmod(dir, 0o444)
	defer os.Chmod(dir, 0o755) // cleanup

	cfg := &SourcesYAML{
		Sources: map[string]SourceEntry{
			"web": {Settings: map[string]string{"urls": "https://example.com"}},
		},
	}
	err := SaveSourcesConfig(dir, cfg)
	if err == nil {
		t.Fatal("expected error saving to read-only directory")
	}
}

func TestSaveSourcesConfig_Overwrite(t *testing.T) {
	dir := t.TempDir()
	// Save initial config.
	cfg1 := &SourcesYAML{
		Sources: map[string]SourceEntry{
			"github": {Settings: map[string]string{"owner": "old"}},
		},
	}
	if err := SaveSourcesConfig(dir, cfg1); err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Overwrite with new config.
	cfg2 := &SourcesYAML{
		Sources: map[string]SourceEntry{
			"jira": {Settings: map[string]string{"project": "NEW"}},
		},
	}
	if err := SaveSourcesConfig(dir, cfg2); err != nil {
		t.Fatalf("second save: %v", err)
	}
	loaded, err := LoadSourcesConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := loaded.Sources["github"]; ok {
		t.Fatal("github should have been overwritten")
	}
	if loaded.Sources["jira"].Settings["project"] != "NEW" {
		t.Fatal("jira project should be NEW")
	}
}

func TestMapYAMLKeys_Jira(t *testing.T) {
	settings := map[string]string{
		"url":     "https://test.atlassian.net",
		"project": "PROJ",
	}
	mapYAMLKeys("jira", settings)

	if settings["base_url"] != "https://test.atlassian.net" {
		t.Errorf("base_url: got %q", settings["base_url"])
	}
	if settings["project_key"] != "PROJ" {
		t.Errorf("project_key: got %q", settings["project_key"])
	}
}

func TestMapYAMLKeys_Linear(t *testing.T) {
	settings := map[string]string{
		"team": "ENG",
	}
	mapYAMLKeys("linear", settings)

	if settings["team_key"] != "ENG" {
		t.Errorf("team_key: got %q", settings["team_key"])
	}
}

func TestMapYAMLKeys_Notion(t *testing.T) {
	settings := map[string]string{
		"database": "abc123",
	}
	mapYAMLKeys("notion", settings)

	if settings["database_id"] != "abc123" {
		t.Errorf("database_id: got %q", settings["database_id"])
	}
}

func TestBuildCredentials(t *testing.T) {
	creds := Credentials{
		GitHubToken: "gh-token",
		JiraToken:   "jira-token",
		JiraEmail:   "user@test.com",
		LinearToken: "lin-token",
		NotionToken: "ntn-token",
		SlackToken:  "xoxb-token",
	}

	tests := []struct {
		name     string
		expected map[string]string
	}{
		{"github", map[string]string{"github_token": "gh-token"}},
		{"jira", map[string]string{"jira_token": "jira-token", "jira_email": "user@test.com"}},
		{"linear", map[string]string{"linear_token": "lin-token"}},
		{"notion", map[string]string{"notion_token": "ntn-token"}},
		{"slack", map[string]string{"slack_token": "xoxb-token"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCredentials(tt.name, creds)
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("%s: got %q, want %q", k, result[k], v)
				}
			}
		})
	}
}

func TestCreateSourceByName(t *testing.T) {
	names := []string{"github", "jira", "linear", "notion", "slack", "web", "local-pdf"}
	for _, name := range names {
		src := createSourceByName(name)
		if src == nil {
			t.Errorf("createSourceByName(%q) returned nil", name)
			continue
		}
		if name == "local-pdf" {
			if src.Name() != "local-pdf" {
				t.Errorf("expected name %q, got %q", "local-pdf", src.Name())
			}
		}
	}

	// Unknown source should return nil.
	if src := createSourceByName("unknown"); src != nil {
		t.Error("expected nil for unknown source")
	}
}

func TestAutoDetectSources(t *testing.T) {
	dir := t.TempDir()

	// Without docs dir — should only have git.
	reg := NewRegistry()
	autoDetectSources(reg, dir, Credentials{})
	// autoDetect doesn't add git (BuildRegistry does that), so it should have 0.
	if len(reg.SourceNames()) != 0 {
		t.Errorf("expected 0 auto-detected sources without docs/, got %d", len(reg.SourceNames()))
	}

	// With docs dir — should also have pdf.
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	reg2 := NewRegistry()
	autoDetectSources(reg2, dir, Credentials{})
	names := reg2.SourceNames()
	if len(names) != 1 || names[0] != "local-pdf" {
		t.Errorf("expected [local-pdf], got %v", names)
	}

	// With GitHub owner/repo — should also register github.
	reg3 := NewRegistry()
	autoDetectSources(reg3, dir, Credentials{
		GitHubOwner: "test-owner",
		GitHubRepo:  "test-repo",
		GitHubToken: "ghp_test",
	})
	names3 := reg3.SourceNames()
	hasGH := false
	hasPDF := false
	for _, n := range names3 {
		if n == "github" {
			hasGH = true
		}
		if n == "local-pdf" {
			hasPDF = true
		}
	}
	if !hasGH {
		t.Errorf("expected github in auto-detected sources, got %v", names3)
	}
	if !hasPDF {
		t.Errorf("expected local-pdf in auto-detected sources (docs/ exists), got %v", names3)
	}
}
