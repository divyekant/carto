package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// buildRootCmd creates a minimal root command with the persistent flags
// that the output layer expects, then attaches exportCmd() as a subcommand.
func buildRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(exportCmd())
	return root
}

func TestExportCmd_StreamsNDJSON(t *testing.T) {
	withCleanEnv(t)

	// Mock Memories server returning 2 entries.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/memories") {
			http.Error(w, "not found", 404)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"memories": []map[string]any{
				{"id": 1, "text": "func main()", "source": "carto/myapp/layer:atoms", "score": 1.0},
				{"id": 2, "text": "type Config struct", "source": "carto/myapp/layer:atoms", "score": 1.0},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	cmd := buildRootCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(new(strings.Builder))
	cmd.SetArgs([]string{"export", "--project", "myapp", "--pretty"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %q", len(lines), buf.String())
	}

	// Parse first line.
	var entry exportEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to parse NDJSON line: %v", err)
	}
	if entry.Text != "func main()" {
		t.Errorf("expected text 'func main()', got %q", entry.Text)
	}
}

func TestExportCmd_JSONEnvelope(t *testing.T) {
	withCleanEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"memories": []map[string]any{
				{"id": 1, "text": "atom1", "source": "carto/proj/layer:atoms", "score": 1.0},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	cmd := buildRootCmd()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(new(strings.Builder))
	cmd.SetArgs([]string{"export", "--project", "proj", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Exported int    `json:"exported"`
			Project  string `json:"project"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(buf.String()), &env); err != nil {
		t.Fatalf("failed to parse envelope: %v\nraw: %s", err, buf.String())
	}
	if !env.OK {
		t.Error("expected ok=true")
	}
	if env.Data.Exported != 1 {
		t.Errorf("expected exported=1, got %d", env.Data.Exported)
	}
	if env.Data.Project != "proj" {
		t.Errorf("expected project=proj, got %q", env.Data.Project)
	}
}

func TestExportCmd_MissingProject_Errors(t *testing.T) {
	cmd := buildRootCmd()
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))
	cmd.SetArgs([]string{"export"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --project")
	}
}

func TestExportCmd_LayerFilter(t *testing.T) {
	withCleanEnv(t)

	var capturedSource string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSource = r.URL.Query().Get("source")
		json.NewEncoder(w).Encode(map[string]any{
			"memories": []map[string]any{
				{"id": 1, "text": "atom", "source": "carto/myapp/api/layer:atoms"},
				{"id": 2, "text": "history", "source": "carto/myapp/api/layer:history"},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	cmd := buildRootCmd()
	out := new(strings.Builder)
	cmd.SetOut(out)
	cmd.SetErr(new(strings.Builder))
	cmd.SetArgs([]string{"export", "--project", "myapp", "--layer", "atoms", "--pretty"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export command failed: %v", err)
	}

	if capturedSource != "carto/myapp/" {
		t.Errorf("expected project source prefix, got %q", capturedSource)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 exported NDJSON line, got %d: %q", len(lines), out.String())
	}

	var entry exportEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("failed to parse NDJSON line: %v", err)
	}
	if entry.Source != "carto/myapp/api/layer:atoms" {
		t.Errorf("expected atoms-only export, got %q", entry.Source)
	}
}
