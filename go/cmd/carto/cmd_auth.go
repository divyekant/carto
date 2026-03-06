package main

// cmd_auth.go — B2B credential management commands.
//
// The auth command group helps operators inspect, set, and validate the
// API keys Carto needs to run. All sensitive values are written to the
// persisted config file (never echoed to stdout in plain text).
//
// Usage:
//
//	carto auth status              # show which credentials are configured
//	carto auth set-key <provider> <key>  # store an API key
//	carto auth validate            # probe the configured LLM provider

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Carto credentials",
		Long: `The auth command group lets you inspect, configure, and validate the
API keys required by Carto without editing config files manually.`,
	}
	cmd.AddCommand(authStatusCmd())
	cmd.AddCommand(authSetKeyCmd())
	cmd.AddCommand(authValidateCmd())
	return cmd
}

// ─── auth status ──────────────────────────────────────────────────────────

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show configured credential status",
		Long: `Displays which API keys and tokens are currently set (masked for security)
and identifies any gaps that would prevent Carto from running.`,
		RunE: runAuthStatus,
	}
}

func runAuthStatus(cmd *cobra.Command, _ []string) error {
	cfg := config.Load()
	profile := resolveProfile(cmd)

	verboseLog(cmd, "loading config for profile %q", profile)

	type credRow struct {
		Name    string `json:"name"`
		Status  string `json:"status"`  // "set" | "unset"
		Masked  string `json:"masked"`  // partial value shown to operator
		Source  string `json:"source"`  // "env" | "file" | ""
		Warning string `json:"warning,omitempty"`
	}

	apiKey := cfg.EffectiveAPIKey()

	rows := []credRow{
		credentialRow("LLM API Key", apiKey, requiredForProvider(cfg.LLMProvider)),
		credentialRow("Anthropic Key", cfg.AnthropicKey, cfg.LLMProvider == "anthropic" || cfg.LLMProvider == ""),
		credentialRow("Memories Key", cfg.MemoriesKey, false),
		credentialRow("GitHub Token", cfg.GitHubToken, false),
		credentialRow("Jira Token", cfg.JiraToken, false),
		credentialRow("Linear Token", cfg.LinearToken, false),
		credentialRow("Notion Token", cfg.NotionToken, false),
		credentialRow("Slack Token", cfg.SlackToken, false),
		credentialRow("Server Token", cfg.ServerToken, false),
	}

	// Annotate warnings.
	for i := range rows {
		if rows[i].Status == "unset" && rows[i].Warning == "" {
			if isRequired(rows[i].Name, cfg) {
				rows[i].Warning = "required for current provider"
			}
		}
	}

	type statusOutput struct {
		Profile  string    `json:"profile"`
		Provider string    `json:"llm_provider"`
		Creds    []credRow `json:"credentials"`
	}

	out := statusOutput{
		Profile:  profile,
		Provider: cfg.LLMProvider,
		Creds:    rows,
	}

	writeOutput(cmd, out, func() {
		fmt.Printf("%s%sAuth Status%s  profile: %s  provider: %s\n\n",
			bold, gold, reset, profile, cfg.LLMProvider)
		fmt.Printf("  %-20s %-8s %-25s %s\n", "CREDENTIAL", "STATUS", "MASKED VALUE", "NOTE")
		fmt.Printf("  %-20s %-8s %-25s %s\n",
			strings.Repeat("-", 20), strings.Repeat("-", 8),
			strings.Repeat("-", 25), strings.Repeat("-", 20))
		for _, r := range rows {
			mark := checkMark(r.Status == "set")
			note := r.Warning
			masked := r.Masked
			if masked == "" {
				masked = "(not set)"
			}
			fmt.Printf("  %s %-18s %-8s %-25s %s\n", mark, r.Name, r.Status, masked, note)
		}
		fmt.Println()

		if apiKey == "" {
			printWarn("No effective LLM API key found. Run: carto auth set-key anthropic <key>")
		} else {
			fmt.Printf("%s%sReady:%s LLM key is set. Run %scarto auth validate%s to test connectivity.\n",
				bold, green, reset, bold, reset)
		}
	})

	logAuditEvent(cmd, "ok", "", map[string]any{"profile": profile})
	return nil
}

// ─── auth set-key ─────────────────────────────────────────────────────────

func authSetKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-key <provider> <api-key>",
		Short: "Store an API key in the persisted config",
		Long: `Saves an API key to the persisted configuration file (never printed to stdout).

Providers:
  anthropic   Anthropic Claude API key (ANTHROPIC_API_KEY)
  openai      OpenAI-compatible API key (LLM_API_KEY)
  memories    Memories vector store API key (MEMORIES_API_KEY)
  github      GitHub personal access token (GITHUB_TOKEN)
  jira        Jira API token (JIRA_TOKEN)
  linear      Linear API key (LINEAR_TOKEN)
  notion      Notion integration token (NOTION_TOKEN)
  slack       Slack bot token (SLACK_TOKEN)
  server      Carto web server Bearer token (CARTO_SERVER_TOKEN)`,
		Args:    cobra.ExactArgs(2),
		RunE:    runAuthSetKey,
		Example: "  carto auth set-key anthropic sk-ant-api03-...",
	}
}

func runAuthSetKey(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])
	key := args[1]

	if key == "" {
		return fmt.Errorf("API key must not be empty")
	}

	cfg := config.Load()

	switch provider {
	case "anthropic":
		cfg.AnthropicKey = key
	case "openai", "llm":
		cfg.LLMApiKey = key
	case "memories":
		cfg.MemoriesKey = key
	case "github":
		cfg.GitHubToken = key
	case "jira":
		cfg.JiraToken = key
	case "linear":
		cfg.LinearToken = key
	case "notion":
		cfg.NotionToken = key
	case "slack":
		cfg.SlackToken = key
	case "server":
		cfg.ServerToken = key
	default:
		return fmt.Errorf("unknown provider %q — valid: anthropic, openai, memories, github, jira, linear, notion, slack, server", provider)
	}

	if config.ConfigPath == "" {
		printWarn("No persisted config path is set. Key stored for this session only.")
		printWarn("To persist, run 'carto serve --projects-dir <dir>' which sets the config path.")
	} else {
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	masked := config.MaskSecret(key)
	writeOutput(cmd, map[string]string{
		"provider": provider,
		"status":   "saved",
		"masked":   masked,
	}, func() {
		fmt.Printf("%s✓%s Stored %s key: %s\n", green, reset, provider, masked)
	})

	logAuditEvent(cmd, "ok", "", map[string]any{"provider": provider})
	return nil
}

// ─── auth validate ────────────────────────────────────────────────────────

func authValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Test connectivity to the configured LLM provider",
		Long: `Makes a lightweight HTTP request to the configured LLM provider endpoint
to verify that the API key is accepted. No LLM tokens are consumed.`,
		RunE: runAuthValidate,
	}
	cmd.Flags().Duration("timeout", 10*time.Second, "Probe timeout")
	return cmd
}

func runAuthValidate(cmd *cobra.Command, _ []string) error {
	cfg := config.Load()
	timeout, _ := cmd.Flags().GetDuration("timeout")

	apiKey := cfg.EffectiveAPIKey()
	provider := cfg.LLMProvider
	if provider == "" {
		provider = "anthropic"
	}

	verboseLog(cmd, "validating provider=%s timeout=%s", provider, timeout)

	if apiKey == "" && provider != "ollama" {
		printError("No API key configured. Run: carto auth set-key %s <key>", provider)
		return fmt.Errorf("%w", errAuthFailure("API key not set"))
	}

	type result struct {
		Provider string `json:"provider"`
		Status   string `json:"status"` // "reachable" | "auth_failed" | "unreachable"
		Latency  string `json:"latency_ms,omitempty"`
		Error    string `json:"error,omitempty"`
	}

	client := &http.Client{Timeout: timeout}
	start := time.Now()

	var probeURL, authHeader string
	switch provider {
	case "anthropic":
		probeURL = "https://api.anthropic.com/v1/models"
		authHeader = "x-api-key"
	case "openai":
		base := cfg.LLMBaseURL
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		probeURL = strings.TrimRight(base, "/") + "/models"
		authHeader = "Authorization"
		apiKey = "Bearer " + apiKey
	case "ollama":
		base := cfg.LLMBaseURL
		if base == "" {
			base = "http://localhost:11434"
		}
		probeURL = strings.TrimRight(base, "/") + "/api/tags"
	default:
		return fmt.Errorf("unknown provider %q", provider)
	}

	verboseLog(cmd, "probing %s", probeURL)

	req, err := http.NewRequest(http.MethodGet, probeURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if authHeader == "Authorization" {
		req.Header.Set("Authorization", apiKey)
	} else if authHeader != "" {
		req.Header.Set(authHeader, apiKey)
	}
	req.Header.Set("User-Agent", "carto/"+config.Version)

	resp, err := client.Do(req)
	latency := fmt.Sprintf("%d", time.Since(start).Milliseconds())

	if err != nil {
		res := result{Provider: provider, Status: "unreachable", Latency: latency, Error: err.Error()}
		writeOutput(cmd, res, func() {
			fmt.Printf("%s✗%s %s unreachable: %v\n", red, reset, provider, err)
		})
		logAuditEvent(cmd, "error", err.Error(), nil)
		return fmt.Errorf("provider unreachable: %w", err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		res := result{Provider: provider, Status: "reachable", Latency: latency}
		writeOutput(cmd, res, func() {
			fmt.Printf("%s✓%s %s API key valid — %sms latency\n", green, reset, provider, latency)
		})
		logAuditEvent(cmd, "ok", "", map[string]any{"provider": provider, "latency_ms": latency})
	case http.StatusUnauthorized, http.StatusForbidden:
		res := result{Provider: provider, Status: "auth_failed", Latency: latency, Error: resp.Status}
		writeOutput(cmd, res, func() {
			fmt.Printf("%s✗%s %s authentication failed (%s)\n", red, reset, provider, resp.Status)
			fmt.Printf("   Tip: Re-run %scarto auth set-key %s <new-key>%s\n", bold, provider, reset)
		})
		logAuditEvent(cmd, "error", "auth_failed", map[string]any{"http_status": resp.StatusCode})
		return fmt.Errorf("authentication failed: %s", resp.Status)
	default:
		res := result{Provider: provider, Status: "reachable", Latency: latency,
			Error: fmt.Sprintf("unexpected status %d", resp.StatusCode)}
		writeOutput(cmd, res, func() {
			fmt.Printf("%s⚠%s %s responded %d — may still work\n", amber, reset, provider, resp.StatusCode)
		})
		logAuditEvent(cmd, "ok", fmt.Sprintf("http_%d", resp.StatusCode), nil)
	}

	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────

// credentialRow builds a display row for a given credential.
func credentialRow(name, val string, required bool) struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Masked  string `json:"masked"`
	Source  string `json:"source"`
	Warning string `json:"warning,omitempty"`
} {
	if val == "" {
		w := ""
		if required {
			w = "required"
		}
		return struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Masked  string `json:"masked"`
			Source  string `json:"source"`
			Warning string `json:"warning,omitempty"`
		}{Name: name, Status: "unset", Warning: w}
	}
	return struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Masked  string `json:"masked"`
		Source  string `json:"source"`
		Warning string `json:"warning,omitempty"`
	}{Name: name, Status: "set", Masked: config.MaskSecret(val)}
}

// requiredForProvider returns true if an API key is mandatory for the given
// provider string.
func requiredForProvider(provider string) bool {
	return provider == "anthropic" || provider == "openai" || provider == ""
}

// isRequired checks whether a named credential is required given the current config.
func isRequired(credName string, cfg config.Config) bool {
	switch credName {
	case "LLM API Key":
		return cfg.LLMProvider == "openai"
	case "Anthropic Key":
		return cfg.LLMProvider == "anthropic" || cfg.LLMProvider == ""
	}
	return false
}

// authFailureError is a sentinel type for auth validation errors.
type authFailureError struct{ msg string }

func (e authFailureError) Error() string { return e.msg }

func errAuthFailure(msg string) error { return authFailureError{msg: msg} }
