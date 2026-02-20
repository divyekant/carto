package sources

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SourcesYAML is the parsed representation of .carto/sources.yaml.
type SourcesYAML struct {
	Sources map[string]SourceEntry `yaml:"sources"`
}

// SourceEntry is a single source definition in the yaml file.
type SourceEntry struct {
	// Flat key-value settings (e.g., "project: PROJ", "url: https://...").
	Settings map[string]string `yaml:"-"`
	// List settings (e.g., "channels: [#eng, #arch]").
	ListSettings map[string][]string `yaml:"-"`
	// Raw holds the unparsed YAML node for flexible parsing.
	Raw map[string]interface{} `yaml:",inline"`
}

// UnmarshalYAML implements custom unmarshalling to separate scalar vs list values.
func (se *SourceEntry) UnmarshalYAML(node *yaml.Node) error {
	// Decode into raw map first.
	var raw map[string]interface{}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	se.Raw = raw
	se.Settings = make(map[string]string)
	se.ListSettings = make(map[string][]string)

	for k, v := range raw {
		switch val := v.(type) {
		case string:
			se.Settings[k] = val
		case bool:
			se.Settings[k] = fmt.Sprintf("%v", val)
		case int:
			se.Settings[k] = fmt.Sprintf("%d", val)
		case float64:
			se.Settings[k] = fmt.Sprintf("%g", val)
		case []interface{}:
			var items []string
			for _, item := range val {
				items = append(items, fmt.Sprintf("%v", item))
			}
			se.ListSettings[k] = items
		}
	}
	return nil
}

// ParseSourcesConfig parses a .carto/sources.yaml file.
func ParseSourcesConfig(data []byte) (*SourcesYAML, error) {
	var cfg SourcesYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("sources config: %w", err)
	}
	if cfg.Sources == nil {
		cfg.Sources = make(map[string]SourceEntry)
	}
	return &cfg, nil
}

// LoadSourcesConfig reads and parses .carto/sources.yaml from the given root directory.
// Returns nil (no error) if the file doesn't exist.
func LoadSourcesConfig(rootPath string) (*SourcesYAML, error) {
	path := filepath.Join(rootPath, ".carto", "sources.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sources config: %w", err)
	}
	return ParseSourcesConfig(data)
}

// Credentials holds all integration tokens/keys from config or environment.
type Credentials struct {
	GitHubToken string
	GitHubOwner string // auto-detected from git URL
	GitHubRepo  string // auto-detected from git URL
	JiraToken   string
	JiraEmail   string
	JiraBaseURL string
	LinearToken string
	NotionToken string
	SlackToken  string
}

// BuildRegistry creates a fully configured source registry by combining:
// 1. Auto-detected sources (git is always registered)
// 2. YAML-configured sources
// 3. Credentials from the app config
func BuildRegistry(rootPath string, yamlCfg *SourcesYAML, creds Credentials) *Registry {
	reg := NewRegistry()

	// Always register git (module-scoped, auto-detected).
	reg.Register(NewGitSource(rootPath))

	if yamlCfg == nil {
		// Auto-detect: register sources based on available credentials.
		autoDetectSources(reg, rootPath, creds)
		return reg
	}

	// Configure sources from YAML.
	for name, entry := range yamlCfg.Sources {
		src := createSourceByName(name)
		if src == nil {
			continue
		}

		// Build SourceConfig from yaml entry + credentials.
		cfg := SourceConfig{
			Settings:    make(map[string]string),
			Credentials: buildCredentials(name, creds),
		}

		// Copy settings from yaml.
		for k, v := range entry.Settings {
			cfg.Settings[k] = v
		}
		// Convert list settings to comma-separated for sources that expect it.
		for k, v := range entry.ListSettings {
			cfg.Settings[k] = strings.Join(v, ",")
		}

		// Map common yaml keys to what sources expect.
		mapYAMLKeys(name, cfg.Settings)

		if err := src.Configure(cfg); err != nil {
			// Skip misconfigured sources.
			continue
		}
		reg.Register(src)
	}

	return reg
}

// createSourceByName returns a new unconfigured source for the given name.
func createSourceByName(name string) Source {
	switch name {
	case "github":
		return NewGitHubSource()
	case "jira":
		return NewJiraSource()
	case "linear":
		return NewLinearSource()
	case "notion":
		return NewNotionSource()
	case "slack":
		return NewSlackSource()
	case "web":
		return NewWebSource()
	case "local-pdf":
		return NewPDFSource()
	default:
		return nil
	}
}

// buildCredentials creates a credentials map for a given source name.
func buildCredentials(name string, creds Credentials) map[string]string {
	m := make(map[string]string)
	switch name {
	case "github":
		if creds.GitHubToken != "" {
			m["github_token"] = creds.GitHubToken
		}
	case "jira":
		if creds.JiraToken != "" {
			m["jira_token"] = creds.JiraToken
		}
		if creds.JiraEmail != "" {
			m["jira_email"] = creds.JiraEmail
		}
	case "linear":
		if creds.LinearToken != "" {
			m["linear_token"] = creds.LinearToken
		}
	case "notion":
		if creds.NotionToken != "" {
			m["notion_token"] = creds.NotionToken
		}
	case "slack":
		if creds.SlackToken != "" {
			m["slack_token"] = creds.SlackToken
		}
	}
	return m
}

// mapYAMLKeys translates user-friendly YAML keys to what each source expects.
func mapYAMLKeys(name string, settings map[string]string) {
	switch name {
	case "jira":
		if v, ok := settings["url"]; ok && settings["base_url"] == "" {
			settings["base_url"] = v
		}
		if v, ok := settings["project"]; ok && settings["project_key"] == "" {
			settings["project_key"] = v
		}
	case "linear":
		if v, ok := settings["team"]; ok && settings["team_key"] == "" {
			settings["team_key"] = v
		}
	case "notion":
		if v, ok := settings["database"]; ok && settings["database_id"] == "" {
			settings["database_id"] = v
		}
	case "slack":
		if v, ok := settings["channels"]; ok && settings["channel_id"] == "" {
			settings["channel_id"] = v
		}
	case "github":
		if v, ok := settings["owner"]; ok {
			settings["owner"] = v
		}
		if v, ok := settings["repo"]; ok {
			settings["repo"] = v
		}
	}
}

// autoDetectSources registers sources based on available credentials
// when no YAML config is present. Sources that require project-specific
// settings (Jira project_key, Linear team_key, Notion database_id,
// Slack channel_id, Web urls) can only be configured via .carto/sources.yaml.
func autoDetectSources(reg *Registry, rootPath string, creds Credentials) {
	// GitHub: register if owner/repo are available (parsed from git URL).
	if creds.GitHubOwner != "" && creds.GitHubRepo != "" {
		ghSrc := NewGitHubSource()
		if err := ghSrc.Configure(SourceConfig{
			Settings:    map[string]string{"owner": creds.GitHubOwner, "repo": creds.GitHubRepo},
			Credentials: buildCredentials("github", creds),
		}); err == nil {
			reg.Register(ghSrc)
		}
	}

	// Auto-detect PDF docs directory.
	docsDir := filepath.Join(rootPath, "docs")
	if info, err := os.Stat(docsDir); err == nil && info.IsDir() {
		pdfSrc := NewPDFSource()
		if err := pdfSrc.Configure(SourceConfig{
			Settings: map[string]string{"dir": docsDir},
		}); err == nil {
			reg.Register(pdfSrc)
		}
	}

	// NOTE: Jira, Linear, Notion, Slack, and Web require project-specific
	// settings (project_key, team_key, database_id, channel_id, urls) that
	// cannot be auto-detected. Use .carto/sources.yaml to configure them.
}
