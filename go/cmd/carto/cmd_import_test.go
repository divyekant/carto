package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newRootWithImport builds a minimal root command with the persistent flags
// that the output layer expects, then attaches importCmd() as a subcommand.
func newRootWithImport() *cobra.Command {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(importCmd())
	return root
}

// =========================================================================
// TestImportCmd_AddStrategy — pipe NDJSON via stdin, verify add-batch called
// =========================================================================

func TestImportCmd_AddStrategy(t *testing.T) {
	withCleanEnv(t)

	batchCalled := 0
	var receivedBodies [][]byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/memory/add-batch":
			batchCalled++
			body, _ := io.ReadAll(r.Body)
			receivedBodies = append(receivedBodies, body)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	ndjson := strings.Join([]string{
		`{"text":"func main() {}","source":"carto/testproj/layer:atoms"}`,
		`{"text":"package main","source":"carto/testproj/layer:atoms"}`,
		``,
		`{"text":"import fmt","source":"carto/testproj/layer:atoms"}`,
	}, "\n")

	root := newRootWithImport()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(ndjson))
	root.SetArgs([]string{"import", "--project", "testproj", "--pretty"})

	if err := root.Execute(); err != nil {
		t.Fatalf("import command failed: %v", err)
	}

	if batchCalled == 0 {
		t.Error("expected add-batch endpoint to be called at least once")
	}

	// Verify all 3 records were sent (empty line should be skipped).
	totalSent := 0
	for _, body := range receivedBodies {
		var payload struct {
			Memories []json.RawMessage `json:"memories"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("invalid batch body: %v", err)
		}
		totalSent += len(payload.Memories)
	}
	if totalSent != 3 {
		t.Errorf("expected 3 memories sent in batches, got %d", totalSent)
	}
}

// =========================================================================
// TestImportCmd_ReplaceStrategy_RequiresConfirmation
// =========================================================================

func TestImportCmd_ReplaceStrategy_RequiresConfirmation(t *testing.T) {
	withCleanEnv(t)

	deleteCalled := false
	batchCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/memory/delete-by-prefix":
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count":5}`))
		case "/memory/add-batch":
			batchCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	ndjson := `{"text":"some data","source":"carto/proj/layer:atoms"}`

	root := newRootWithImport()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(ndjson))
	// No --yes flag, and in test environment isJSONMode returns true (non-TTY),
	// so confirmAction returns false and the import should be cancelled.
	root.SetArgs([]string{"import", "--project", "proj", "--strategy", "replace"})

	if err := root.Execute(); err != nil {
		t.Fatalf("import command failed: %v", err)
	}

	if deleteCalled {
		t.Error("delete-by-prefix should NOT have been called without --yes confirmation")
	}
	if batchCalled {
		t.Error("add-batch should NOT have been called when import is cancelled")
	}
}

// =========================================================================
// TestImportCmd_ReplaceStrategy_WithYes — deletes then imports
// =========================================================================

func TestImportCmd_ReplaceStrategy_WithYes(t *testing.T) {
	withCleanEnv(t)

	deleteCalled := false
	var deleteBody []byte
	batchCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/memory/delete-by-prefix":
			deleteCalled = true
			deleteBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count":10}`))
		case "/memory/add-batch":
			batchCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	ndjson := `{"text":"replaced data","source":"carto/myproj/layer:atoms"}`

	root := newRootWithImport()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(ndjson))
	root.SetArgs([]string{"--yes", "import", "--project", "myproj", "--strategy", "replace"})

	if err := root.Execute(); err != nil {
		t.Fatalf("import command failed: %v", err)
	}

	if !deleteCalled {
		t.Error("expected delete-by-prefix endpoint to be called with --yes")
	}

	// Verify delete used the correct source prefix.
	var delPayload struct {
		SourcePrefix string `json:"source_prefix"`
	}
	if err := json.Unmarshal(deleteBody, &delPayload); err != nil {
		t.Fatalf("invalid delete body: %v", err)
	}
	if delPayload.SourcePrefix != "carto/myproj/" {
		t.Errorf("expected source_prefix %q, got %q", "carto/myproj/", delPayload.SourcePrefix)
	}

	if !batchCalled {
		t.Error("expected add-batch endpoint to be called after delete")
	}
}

// =========================================================================
// TestImportCmd_MissingProject_Errors
// =========================================================================

func TestImportCmd_MissingProject_Errors(t *testing.T) {
	root := newRootWithImport()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"import"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --project is missing")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Errorf("expected error mentioning 'project', got: %v", err)
	}
}

// =========================================================================
// TestImportCmd_JSONEnvelope — verify envelope output with --json flag
// =========================================================================

func TestImportCmd_JSONEnvelope(t *testing.T) {
	withCleanEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/memory/add-batch":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	ndjson := strings.Join([]string{
		`{"text":"entry one","source":"carto/envproj/layer:atoms"}`,
		`{"text":"entry two","source":"carto/envproj/layer:wiring"}`,
	}, "\n")

	root := newRootWithImport()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(ndjson))
	root.SetArgs([]string{"--json", "import", "--project", "envproj"})

	if err := root.Execute(); err != nil {
		t.Fatalf("import --json failed: %v", err)
	}

	// Parse the JSON envelope from stdout.
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Imported int    `json:"imported"`
			Project  string `json:"project"`
			Strategy string `json:"strategy"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal(stdout.Bytes(), &env); jsonErr != nil {
		t.Fatalf("not valid JSON: %v\nraw stdout: %s\nraw stderr: %s", jsonErr, stdout.String(), stderr.String())
	}
	if !env.OK {
		t.Error("expected ok=true in envelope")
	}
	if env.Data.Imported != 2 {
		t.Errorf("expected imported=2, got %d", env.Data.Imported)
	}
	if env.Data.Project != "envproj" {
		t.Errorf("expected project=envproj, got %q", env.Data.Project)
	}
	if env.Data.Strategy != "add" {
		t.Errorf("expected strategy=add, got %q", env.Data.Strategy)
	}
}
