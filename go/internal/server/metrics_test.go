package server

// metrics_test.go — QA tests for the GET /api/metrics endpoint.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

// TestMetricsEndpoint_ReturnsJSON verifies that /api/metrics returns a 200
// response with a well-formed JSON body containing required observability fields.
func TestMetricsEndpoint_ReturnsJSON(t *testing.T) {
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, "/tmp/projects", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/metrics, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header on /api/metrics")
	}

	var resp metricsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode /api/metrics: %v", err)
	}

	// Required fields.
	if resp.Version == "" {
		t.Error("expected non-empty version in metrics")
	}
	if resp.UptimeSeconds < 0 {
		t.Error("uptime_seconds must be non-negative")
	}
	if resp.GoRoutines <= 0 {
		t.Error("go_routines must be > 0")
	}
	if resp.MemAllocMB < 0 {
		t.Error("mem_alloc_mb must be non-negative")
	}
}

// TestMetricsEndpoint_AuthEnabled reflects config.
func TestMetricsEndpoint_AuthEnabled_WhenTokenSet(t *testing.T) {
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	cfg := config.Config{ServerToken: "super-secret"}
	srv := New(cfg, memoriesClient, "", nil)

	// Must send the Bearer token for /api/ paths.
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	req.Header.Set("Authorization", "Bearer super-secret")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d: %s", w.Code, w.Body.String())
	}

	var resp metricsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.AuthEnabled {
		t.Error("expected auth_enabled=true when ServerToken is set")
	}
}

// TestMetricsEndpoint_NoAuth_WhenNoToken verifies auth_enabled=false when
// no server token is configured.
func TestMetricsEndpoint_NoAuth_WhenNoToken(t *testing.T) {
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp metricsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.AuthEnabled {
		t.Error("expected auth_enabled=false when no ServerToken is configured")
	}
}

// TestMetricsEndpoint_ActiveRunsCount reflects in-progress index runs.
func TestMetricsEndpoint_ActiveRunsCount(t *testing.T) {
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, "", nil)

	// Baseline.
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp metricsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	baseline := resp.ActiveRuns

	// Start a run.
	run := srv.runs.Start("metrics-test")
	if run == nil {
		t.Fatal("expected to start run")
	}
	t.Cleanup(func() { srv.runs.Finish("metrics-test") })

	req2 := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	var resp2 metricsResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if resp2.ActiveRuns <= baseline {
		t.Errorf("expected more active runs after starting one, got %d (baseline %d)", resp2.ActiveRuns, baseline)
	}
}

// TestBrowse_PathRestriction_Forbids paths outside projectsDir.
func TestBrowse_PathRestriction_Forbids(t *testing.T) {
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	// Set a projects dir to enable path restriction.
	srv := New(config.Config{}, memoriesClient, "/tmp/allowed-projects", nil)

	// Request a path outside /tmp/allowed-projects.
	req := httptest.NewRequest(http.MethodGet, "/api/browse?path=/etc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for restricted path /etc, got %d: %s", w.Code, w.Body.String())
	}
}

// TestBrowse_PathRestriction_AllowsProjectsDir verifies that projectsDir itself is allowed.
func TestBrowse_PathRestriction_AllowsProjectsDir(t *testing.T) {
	// Use a real directory so Stat succeeds.
	tmpDir := t.TempDir()
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, tmpDir, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/browse?path="+tmpDir, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should succeed — tmpDir is the projectsDir.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for projectsDir path, got %d: %s", w.Code, w.Body.String())
	}
}
