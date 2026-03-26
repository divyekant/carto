package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMemoriesClient_Health(t *testing.T) {
	t.Run("healthy server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer srv.Close()

		client := NewMemoriesClient(srv.URL, "test-key")
		ok, err := client.Health()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Error("expected healthy=true")
		}
	})

	t.Run("unhealthy server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		client := NewMemoriesClient(srv.URL, "test-key")
		ok, err := client.Health()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected healthy=false for 503")
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		client := NewMemoriesClient("http://127.0.0.1:1", "test-key")
		ok, err := client.Health()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected healthy=false for unreachable server")
		}
	})
}

func TestMemoriesClient_AddMemory(t *testing.T) {
	var receivedAPIKey string
	var receivedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/add" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		receivedAPIKey = r.Header.Get("X-API-Key")

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":42}`))
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "secret-key-123")
	id, err := client.AddMemory(Memory{
		Text:     "Go is great",
		Source:   "test/lang",
		Metadata: map[string]any{"lang": "go"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAPIKey != "secret-key-123" {
		t.Errorf("expected API key 'secret-key-123', got '%s'", receivedAPIKey)
	}

	if id != 42 {
		t.Errorf("expected id=42, got %d", id)
	}

	if receivedBody["text"] != "Go is great" {
		t.Errorf("expected text 'Go is great', got '%v'", receivedBody["text"])
	}
	if receivedBody["source"] != "test/lang" {
		t.Errorf("expected source 'test/lang', got '%v'", receivedBody["source"])
	}
}

func TestMemoriesClient_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		if body["query"] != "test query" {
			t.Errorf("expected query 'test query', got '%v'", body["query"])
		}
		if body["k"] != float64(5) {
			t.Errorf("expected k=5, got %v", body["k"])
		}
		if body["hybrid"] != true {
			t.Errorf("expected hybrid=true, got %v", body["hybrid"])
		}

		resp := map[string]any{
			"results": []map[string]any{
				{"id": 1, "text": "result one", "score": 0.95, "source": "src/a", "metadata": map[string]any{"key": "val"}},
				{"id": 2, "text": "result two", "score": 0.80, "source": "src/b"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	results, err := client.Search("test query", SearchOptions{
		K:      5,
		Hybrid: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != 1 {
		t.Errorf("expected first result id=1, got %d", results[0].ID)
	}
	if results[0].Text != "result one" {
		t.Errorf("expected first result text='result one', got '%s'", results[0].Text)
	}
	if results[0].Score != 0.95 {
		t.Errorf("expected first result score=0.95, got %f", results[0].Score)
	}
	if results[0].Source != "src/a" {
		t.Errorf("expected first result source='src/a', got '%s'", results[0].Source)
	}
	if results[0].Metadata["key"] != "val" {
		t.Errorf("expected first result meta key=val, got %v", results[0].Metadata)
	}
	if results[1].ID != 2 {
		t.Errorf("expected second result id=2, got %d", results[1].ID)
	}
}

func TestMemoriesClient_DeleteBySource(t *testing.T) {
	var receivedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/memory/delete-by-prefix" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"count": 15})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	count, err := client.DeleteBySource("proj/old")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 15 {
		t.Errorf("expected 15 deleted, got %d", count)
	}

	if receivedBody["source_prefix"] != "proj/old" {
		t.Errorf("expected source_prefix 'proj/old', got '%v'", receivedBody["source_prefix"])
	}
}

func TestMemoriesClient_Count(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/memories/count" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		source := r.URL.Query().Get("source")
		if source != "carto/proj/" {
			t.Errorf("expected source 'carto/proj/', got '%s'", source)
		}

		json.NewEncoder(w).Encode(map[string]any{"count": 42})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	count, err := client.Count("carto/proj/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 42 {
		t.Errorf("expected count 42, got %d", count)
	}
}

func TestMemoriesClient_ListBySource_WithOffset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/memories" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		source := r.URL.Query().Get("source")
		limit := r.URL.Query().Get("limit")
		offset := r.URL.Query().Get("offset")

		if source != "carto/proj/mod/layer:atoms" {
			t.Errorf("expected source 'carto/proj/mod/layer:atoms', got '%s'", source)
		}
		if limit != "50" {
			t.Errorf("expected limit '50', got '%s'", limit)
		}
		if offset != "100" {
			t.Errorf("expected offset '100', got '%s'", offset)
		}

		resp := map[string]any{
			"memories": []map[string]any{
				{"id": 101, "text": "atom 101", "score": 1.0, "source": source},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	results, err := client.ListBySource("carto/proj/mod/layer:atoms", 50, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != 101 {
		t.Errorf("expected id 101, got %d", results[0].ID)
	}
}

func TestMemoriesClient_Search_WithSourcePrefix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		if body["source_prefix"] != "carto/proj/" {
			t.Errorf("expected source_prefix 'carto/proj/', got '%v'", body["source_prefix"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 1, "text": "match", "score": 0.9, "source": "carto/proj/a"},
			},
		})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	results, err := client.Search("test", SearchOptions{
		K:            5,
		SourcePrefix: "carto/proj/",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestMemoriesClient_UpsertBatch(t *testing.T) {
	var receivedBatches []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/memory/upsert-batch" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		receivedBatches = append(receivedBatches, body)

		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 100, "status": "created"},
				{"id": 101, "status": "updated"},
			},
		})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	results, err := client.UpsertBatch([]Memory{
		{Text: "first", Source: "test/a"},
		{Text: "second", Source: "test/b"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != 100 {
		t.Errorf("expected first id=100, got %d", results[0].ID)
	}
	if results[0].Status != "created" {
		t.Errorf("expected first status=created, got %s", results[0].Status)
	}
	if results[1].ID != 101 {
		t.Errorf("expected second id=101, got %d", results[1].ID)
	}
	if results[1].Status != "updated" {
		t.Errorf("expected second status=updated, got %s", results[1].Status)
	}
}

func TestMemoriesClient_Supersede(t *testing.T) {
	var receivedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/memory/supersede" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		json.NewEncoder(w).Encode(map[string]any{"new_id": 99})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	newID, err := client.Supersede(42, "updated text", map[string]any{"version": 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if newID != 99 {
		t.Errorf("expected new_id=99, got %d", newID)
	}
	if receivedBody["old_id"] != float64(42) {
		t.Errorf("expected old_id=42, got %v", receivedBody["old_id"])
	}
	if receivedBody["text"] != "updated text" {
		t.Errorf("expected text='updated text', got '%v'", receivedBody["text"])
	}
}

func TestMemoriesClient_CreateLink(t *testing.T) {
	var receivedBody map[string]any
	var receivedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	err := client.CreateLink(10, 20, "depends_on")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedPath != "/memory/10/link" {
		t.Errorf("expected path '/memory/10/link', got '%s'", receivedPath)
	}
	if receivedBody["to_id"] != float64(20) {
		t.Errorf("expected to_id=20, got %v", receivedBody["to_id"])
	}
	if receivedBody["type"] != "depends_on" {
		t.Errorf("expected type='depends_on', got '%v'", receivedBody["type"])
	}
}

func TestMemoriesClient_GetLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/memory/10/links" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"links": []map[string]any{
				{"to_id": 20, "type": "depends_on", "created_at": "2026-01-01T00:00:00Z"},
				{"to_id": 30, "type": "related_to", "created_at": "2026-01-02T00:00:00Z"},
			},
		})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	links, err := client.GetLinks(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].ToID != 20 {
		t.Errorf("expected first link to_id=20, got %d", links[0].ToID)
	}
	if links[0].Type != "depends_on" {
		t.Errorf("expected first link type='depends_on', got '%s'", links[0].Type)
	}
	if links[1].ToID != 30 {
		t.Errorf("expected second link to_id=30, got %d", links[1].ToID)
	}
}

func TestMemoriesClient_DeleteLinks(t *testing.T) {
	var deletedPaths []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/memory/10/links":
			json.NewEncoder(w).Encode(map[string]any{
				"links": []map[string]any{
					{"to_id": 20, "type": "depends_on"},
					{"to_id": 30, "type": "related_to"},
				},
			})
		case r.Method == http.MethodDelete:
			deletedPaths = append(deletedPaths, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	err := client.DeleteLinks(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deletedPaths) != 2 {
		t.Fatalf("expected 2 deletes, got %d", len(deletedPaths))
	}
	if deletedPaths[0] != "/memory/10/link/20" {
		t.Errorf("expected delete path '/memory/10/link/20', got '%s'", deletedPaths[0])
	}
	if deletedPaths[1] != "/memory/10/link/30" {
		t.Errorf("expected delete path '/memory/10/link/30', got '%s'", deletedPaths[1])
	}
}
