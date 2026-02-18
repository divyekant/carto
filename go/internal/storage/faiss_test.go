package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFaissClient_Health(t *testing.T) {
	t.Run("healthy server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}))
		defer srv.Close()

		client := NewFaissClient(srv.URL, "test-key")
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

		client := NewFaissClient(srv.URL, "test-key")
		ok, err := client.Health()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected healthy=false for 503")
		}
	})

	t.Run("unreachable server", func(t *testing.T) {
		client := NewFaissClient("http://127.0.0.1:1", "test-key")
		ok, err := client.Health()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected healthy=false for unreachable server")
		}
	})
}

func TestFaissClient_AddMemory(t *testing.T) {
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

	client := NewFaissClient(srv.URL, "secret-key-123")
	id, err := client.AddMemory(Memory{
		Text:        "Go is great",
		Source:      "test/lang",
		Metadata:    map[string]any{"lang": "go"},
		Deduplicate: true,
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
	if receivedBody["deduplicate"] != true {
		t.Errorf("expected deduplicate=true, got %v", receivedBody["deduplicate"])
	}
}

func TestFaissClient_Search(t *testing.T) {
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

	client := NewFaissClient(srv.URL, "test-key")
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
	if results[0].Meta["key"] != "val" {
		t.Errorf("expected first result meta key=val, got %v", results[0].Meta)
	}
	if results[1].ID != 2 {
		t.Errorf("expected second result id=2, got %d", results[1].ID)
	}
}

func TestFaissClient_DeleteBySource(t *testing.T) {
	var deletedIDs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/memories":
			source := r.URL.Query().Get("source")
			if source != "proj/old" {
				t.Errorf("expected source 'proj/old', got '%s'", source)
			}
			resp := map[string]any{
				"memories": []map[string]any{
					{"id": 10, "text": "old one", "score": 1.0, "source": "proj/old"},
					{"id": 20, "text": "old two", "score": 1.0, "source": "proj/old"},
					{"id": 30, "text": "old three", "score": 1.0, "source": "proj/old"},
				},
			}
			json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/memory/"):
			id := strings.TrimPrefix(r.URL.Path, "/memory/")
			deletedIDs = append(deletedIDs, id)
			w.WriteHeader(http.StatusOK)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewFaissClient(srv.URL, "test-key")
	count, err := client.DeleteBySource("proj/old")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 deleted, got %d", count)
	}
	if len(deletedIDs) != 3 {
		t.Fatalf("expected 3 delete calls, got %d", len(deletedIDs))
	}

	expected := []string{"10", "20", "30"}
	for i, id := range expected {
		if deletedIDs[i] != id {
			t.Errorf("expected delete ID %s at position %d, got %s", id, i, deletedIDs[i])
		}
	}
}
