package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newRootWithQuery() *cobra.Command {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(queryCmd())
	return root
}

func TestQueryCmd_ProjectTierSearch(t *testing.T) {
	withCleanEnv(t)

	var captured struct {
		Query        string `json:"query"`
		SourcePrefix string `json:"source_prefix"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode search body: %v", err)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": 1, "text": "zones", "score": 0.9, "source": "carto/myproj/web/layer:zones"},
					{"id": 2, "text": "blueprint", "score": 0.8, "source": "carto/myproj/_system/layer:blueprint"},
					{"id": 3, "text": "history", "score": 0.7, "source": "carto/myproj/web/layer:history"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("MEMORIES_URL", srv.URL)

	root := newRootWithQuery()
	var out, errOut strings.Builder
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"--json", "query", "auth", "--project", "myproj", "--tier", "standard", "-k", "5"})

	if err := root.Execute(); err != nil {
		t.Fatalf("query command failed: %v", err)
	}

	if captured.Query != "auth" {
		t.Fatalf("expected search query auth, got %q", captured.Query)
	}
	if captured.SourcePrefix != "carto/myproj/" {
		t.Fatalf("expected source_prefix carto/myproj/, got %q", captured.SourcePrefix)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data []struct {
			Source string `json:"source"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("decode output envelope: %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok=true envelope, got %s", out.String())
	}
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 tier-filtered results, got %d: %s", len(env.Data), out.String())
	}
	for _, item := range env.Data {
		if strings.Contains(item.Source, "layer:history") {
			t.Fatalf("history layer should not be included for standard tier: %q", item.Source)
		}
	}
}
