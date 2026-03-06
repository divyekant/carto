package main

// cmd_doctor.go — pre-flight environment health checks for B2B operators.
//
// Running `carto doctor` before deploying or onboarding a new environment
// gives instant feedback on which required settings are missing or broken.

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

// checkStatus describes the outcome of a single doctor check.
type checkStatus int

const (
	checkOK   checkStatus = iota // requirement met
	checkWarn                    // advisory / optional
	checkFail                    // requirement not met
)

// doctorCheck holds the result of a single pre-flight check.
type doctorCheck struct {
	Name    string      `json:"name"`
	Status  string      `json:"status"` // "ok" | "warn" | "fail"
	Message string      `json:"message"`
	Hint    string      `json:"hint,omitempty"`
	raw     checkStatus // not serialised
}

func (c doctorCheck) icon() string {
	switch c.raw {
	case checkOK:
		return green + "✓" + reset
	case checkWarn:
		return amber + "⚠" + reset
	default:
		return red + "✗" + reset
	}
}

func doctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run pre-flight environment checks",
		Long: `doctor inspects the current environment and reports which requirements are
met, which are optional (warn), and which are missing (fail).

Exit code is non-zero when any check fails.`,
		RunE: runDoctor,
	}
	cmd.Flags().Duration("timeout", 8*time.Second, "Timeout for network probe checks")
	cmd.Flags().Bool("skip-network", false, "Skip network connectivity checks")
	return cmd
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	cfg := config.Load()
	timeout, _ := cmd.Flags().GetDuration("timeout")
	skipNetwork, _ := cmd.Flags().GetBool("skip-network")
	profile := resolveProfile(cmd)

	verboseLog(cmd, "running doctor checks, profile=%s timeout=%s", profile, timeout)

	var checks []doctorCheck

	// ── 1. LLM API key ────────────────────────────────────────────────────
	apiKey := cfg.EffectiveAPIKey()
	if apiKey == "" && cfg.LLMProvider != "ollama" {
		checks = append(checks, doctorCheck{
			Name:    "LLM API Key",
			raw:     checkFail,
			Status:  "fail",
			Message: "No API key found for provider " + cfg.LLMProvider,
			Hint:    "Set ANTHROPIC_API_KEY or LLM_API_KEY, or run: carto auth set-key <provider> <key>",
		})
	} else {
		label := config.MaskSecret(apiKey)
		if apiKey == "" {
			label = "(ollama — key not required)"
		}
		checks = append(checks, doctorCheck{
			Name:    "LLM API Key",
			raw:     checkOK,
			Status:  "ok",
			Message: cfg.LLMProvider + " — " + label,
		})
	}

	// ── 2. LLM provider ───────────────────────────────────────────────────
	switch cfg.LLMProvider {
	case "anthropic", "openai", "ollama", "":
		checks = append(checks, doctorCheck{
			Name:    "LLM Provider",
			raw:     checkOK,
			Status:  "ok",
			Message: cfg.LLMProvider + " (fast=" + cfg.FastModel + ", deep=" + cfg.DeepModel + ")",
		})
	default:
		checks = append(checks, doctorCheck{
			Name:    "LLM Provider",
			raw:     checkWarn,
			Status:  "warn",
			Message: "unrecognised provider: " + cfg.LLMProvider,
			Hint:    "Set LLM_PROVIDER to anthropic, openai, or ollama",
		})
	}

	// ── 3. PROJECTS_DIR ───────────────────────────────────────────────────
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		checks = append(checks, doctorCheck{
			Name:    "PROJECTS_DIR",
			raw:     checkWarn,
			Status:  "warn",
			Message: "not set — multi-project features disabled",
			Hint:    "Export PROJECTS_DIR pointing to a writable directory",
		})
	} else {
		info, err := os.Stat(projectsDir)
		if err != nil || !info.IsDir() {
			checks = append(checks, doctorCheck{
				Name:    "PROJECTS_DIR",
				raw:     checkFail,
				Status:  "fail",
				Message: "directory does not exist or is not accessible: " + projectsDir,
				Hint:    "Create the directory or fix the path: mkdir -p " + projectsDir,
			})
		} else {
			// Check writability by attempting a temp file.
			tmpFile := fmt.Sprintf("%s/.carto-doctor-%d", projectsDir, os.Getpid())
			if f, err := os.Create(tmpFile); err == nil {
				f.Close()
				os.Remove(tmpFile)
				checks = append(checks, doctorCheck{
					Name:    "PROJECTS_DIR",
					raw:     checkOK,
					Status:  "ok",
					Message: projectsDir + " (writable)",
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:    "PROJECTS_DIR",
					raw:     checkFail,
					Status:  "fail",
					Message: "directory is not writable: " + projectsDir,
					Hint:    "Fix permissions: chmod u+w " + projectsDir,
				})
			}
		}
	}

	// ── 4. Config file ────────────────────────────────────────────────────
	cfgDir := config.ConfigDir()
	if _, err := os.Stat(cfgDir); os.IsNotExist(err) {
		checks = append(checks, doctorCheck{
			Name:    "Config Directory",
			raw:     checkWarn,
			Status:  "warn",
			Message: cfgDir + " does not exist yet",
			Hint:    "It will be created automatically on first use",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "Config Directory",
			raw:     checkOK,
			Status:  "ok",
			Message: cfgDir,
		})
	}

	// ── 5. Audit log ──────────────────────────────────────────────────────
	if cfg.AuditLogFile == "" {
		checks = append(checks, doctorCheck{
			Name:    "Audit Log",
			raw:     checkWarn,
			Status:  "warn",
			Message: "audit logging is disabled",
			Hint:    "Set CARTO_AUDIT_LOG=/path/to/carto-audit.log to enable",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "Audit Log",
			raw:     checkOK,
			Status:  "ok",
			Message: cfg.AuditLogFile,
		})
	}

	// ── 6. Server auth ────────────────────────────────────────────────────
	if cfg.ServerToken == "" {
		checks = append(checks, doctorCheck{
			Name:    "Server Auth",
			raw:     checkWarn,
			Status:  "warn",
			Message: "CARTO_SERVER_TOKEN not set — web UI has no authentication",
			Hint:    "Set CARTO_SERVER_TOKEN to a strong random value for production",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "Server Auth",
			raw:     checkOK,
			Status:  "ok",
			Message: "Bearer token configured (" + config.MaskSecret(cfg.ServerToken) + ")",
		})
	}

	// ── 7. Docker environment ─────────────────────────────────────────────
	if config.IsDocker() {
		checks = append(checks, doctorCheck{
			Name:    "Runtime",
			raw:     checkOK,
			Status:  "ok",
			Message: "running inside Docker container",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:    "Runtime",
			raw:     checkOK,
			Status:  "ok",
			Message: "running natively",
		})
	}

	// ── 8. Network: Memories server ───────────────────────────────────────
	if !skipNetwork {
		memClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)
		memOK, memErr := probeWithTimeout(func() (bool, error) {
			return memClient.Health()
		}, timeout)

		if memErr != nil || !memOK {
			errMsg := "unreachable"
			if memErr != nil {
				errMsg = memErr.Error()
			}
			checks = append(checks, doctorCheck{
				Name:    "Memories Server",
				raw:     checkFail,
				Status:  "fail",
				Message: cfg.MemoriesURL + " — " + errMsg,
				Hint:    "Ensure the Memories service is running: docker run -p 8900:8900 ...",
			})
		} else {
			checks = append(checks, doctorCheck{
				Name:    "Memories Server",
				raw:     checkOK,
				Status:  "ok",
				Message: cfg.MemoriesURL + " — healthy",
			})
		}

		// ── 9. Network: LLM provider (quick HEAD probe) ────────────────────
		if apiKey != "" {
			llmOK, llmErr := probeLLMEndpoint(cfg, timeout)
			if llmErr != nil {
				checks = append(checks, doctorCheck{
					Name:    "LLM Connectivity",
					raw:     checkWarn,
					Status:  "warn",
					Message: "probe failed: " + llmErr.Error(),
					Hint:    "Run 'carto auth validate' for detailed diagnostics",
				})
			} else if !llmOK {
				checks = append(checks, doctorCheck{
					Name:    "LLM Connectivity",
					raw:     checkWarn,
					Status:  "warn",
					Message: "provider responded with unexpected status",
					Hint:    "Run 'carto auth validate' for detailed diagnostics",
				})
			} else {
				checks = append(checks, doctorCheck{
					Name:    "LLM Connectivity",
					raw:     checkOK,
					Status:  "ok",
					Message: cfg.LLMProvider + " endpoint reachable",
				})
			}
		}
	} else {
		verboseLog(cmd, "skipping network checks (--skip-network)")
	}

	// ── Render output ─────────────────────────────────────────────────────
	failures := 0
	warnings := 0
	for _, c := range checks {
		if c.raw == checkFail {
			failures++
		} else if c.raw == checkWarn {
			warnings++
		}
	}

	type doctorOut struct {
		Profile  string        `json:"profile"`
		Checks   []doctorCheck `json:"checks"`
		Failures int           `json:"failures"`
		Warnings int           `json:"warnings"`
	}

	out := doctorOut{
		Profile:  profile,
		Checks:   checks,
		Failures: failures,
		Warnings: warnings,
	}

	writeOutput(cmd, out, func() {
		fmt.Printf("%s%sCarto Doctor%s  profile: %s\n\n", bold, gold, reset, profile)
		for _, c := range checks {
			fmt.Printf("  %s %-22s %s\n", c.icon(), c.Name, c.Message)
			if c.Hint != "" {
				fmt.Printf("      %s↳%s %s\n", amber, reset, c.Hint)
			}
		}
		fmt.Println()

		switch {
		case failures > 0:
			fmt.Printf("%s%s%d check(s) failed.%s Fix the issues above before running Carto in production.\n",
				bold, red, failures, reset)
		case warnings > 0:
			fmt.Printf("%s%s%d warning(s).%s %sEverything required is set; review warnings for hardening.%s\n",
				bold, amber, warnings, reset, amber, reset)
		default:
			fmt.Printf("%s%sAll checks passed.%s Carto is ready.\n", bold, green, reset)
		}
	})

	logAuditEvent(cmd, "ok", "", map[string]any{
		"profile":  profile,
		"failures": failures,
		"warnings": warnings,
	})

	if failures > 0 {
		return fmt.Errorf("%d doctor check(s) failed", failures)
	}
	return nil
}

// ─── network probe helpers ─────────────────────────────────────────────────

// probeWithTimeout runs fn in a goroutine and returns within the given timeout.
func probeWithTimeout(fn func() (bool, error), timeout time.Duration) (bool, error) {
	type probeResult struct {
		ok  bool
		err error
	}
	ch := make(chan probeResult, 1)
	go func() {
		ok, err := fn()
		ch <- probeResult{ok, err}
	}()
	select {
	case r := <-ch:
		return r.ok, r.err
	case <-time.After(timeout):
		return false, fmt.Errorf("timed out after %s", timeout)
	}
}

// probeLLMEndpoint makes a lightweight GET to the LLM provider's models
// endpoint to test reachability. Returns (reachable, error).
func probeLLMEndpoint(cfg config.Config, timeout time.Duration) (bool, error) {
	var url string
	switch cfg.LLMProvider {
	case "anthropic", "":
		url = "https://api.anthropic.com/v1/models"
	case "openai":
		base := cfg.LLMBaseURL
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		url = strings.TrimRight(base, "/") + "/models"
	case "ollama":
		base := cfg.LLMBaseURL
		if base == "" {
			base = "http://localhost:11434"
		}
		url = strings.TrimRight(base, "/") + "/api/tags"
	default:
		return false, fmt.Errorf("unknown provider %q", cfg.LLMProvider)
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	apiKey := cfg.EffectiveAPIKey()
	switch cfg.LLMProvider {
	case "anthropic", "":
		req.Header.Set("x-api-key", apiKey)
	case "openai":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("User-Agent", "carto/"+config.Version)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	return resp.StatusCode < 500, nil
}
