package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

var _ Source = (*SlackSource)(nil)

func TestSlackSource_Name(t *testing.T) {
	src := NewSlackSource()
	if src.Name() != "slack" {
		t.Errorf("Name() = %q, want %q", src.Name(), "slack")
	}
}

func TestSlackSource_Scope(t *testing.T) {
	src := NewSlackSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestSlackSource_Configure(t *testing.T) {
	src := NewSlackSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"channel_id": "C12345"},
		Credentials: map[string]string{"slack_token": "xoxb-test-token"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if src.channelID != "C12345" {
		t.Errorf("channelID = %q, want %q", src.channelID, "C12345")
	}
	if src.token != "xoxb-test-token" {
		t.Errorf("token = %q, want %q", src.token, "xoxb-test-token")
	}
}

func TestSlackSource_Configure_MissingChannel(t *testing.T) {
	src := NewSlackSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{},
		Credentials: map[string]string{"slack_token": "xoxb-test"},
	})
	if err == nil {
		t.Error("expected error when channel_id is missing")
	}
}

func TestSlackSource_Fetch(t *testing.T) {
	// Set up mock Slack API responses.
	historyResp := slackHistoryResponse{
		OK: true,
		Messages: []slackMessage{
			{
				TS:         "1700000000.000001",
				Text:       "This is a standalone message with no thread",
				User:       "U111",
				ReplyCount: 0,
			},
			{
				TS:         "1700000001.000002",
				ThreadTS:   "1700000001.000002",
				Text:       "Thread starter: discussing architecture decisions for the new module",
				User:       "U222",
				ReplyCount: 2,
			},
		},
	}

	repliesResp := slackRepliesResponse{
		OK: true,
		Messages: []slackMessage{
			{
				TS:       "1700000001.000002",
				ThreadTS: "1700000001.000002",
				Text:     "Thread starter: discussing architecture decisions for the new module",
				User:     "U222",
			},
			{
				TS:       "1700000002.000003",
				ThreadTS: "1700000001.000002",
				Text:     "I think we should use the adapter pattern",
				User:     "U333",
			},
			{
				TS:       "1700000003.000004",
				ThreadTS: "1700000001.000002",
				Text:     "Agreed, that keeps things flexible",
				User:     "U444",
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/conversations.history", func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer xoxb-test-token" {
			t.Errorf("unexpected Authorization header: %q", auth)
		}
		// Verify query params.
		if r.URL.Query().Get("channel") != "C12345" {
			t.Errorf("unexpected channel param: %q", r.URL.Query().Get("channel"))
		}
		json.NewEncoder(w).Encode(historyResp)
	})
	mux.HandleFunc("/conversations.replies", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ts") != "1700000001.000002" {
			t.Errorf("unexpected ts param: %q", r.URL.Query().Get("ts"))
		}
		json.NewEncoder(w).Encode(repliesResp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewSlackSource()
	src.baseURL = srv.URL
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"channel_id": "C12345"},
		Credentials: map[string]string{"slack_token": "xoxb-test-token"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test-project"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify standalone message.
	standalone := artifacts[0]
	if standalone.Source != "slack" {
		t.Errorf("standalone.Source = %q, want %q", standalone.Source, "slack")
	}
	if standalone.Category != Context {
		t.Errorf("standalone.Category = %q, want %q", standalone.Category, Context)
	}
	if standalone.ID != "1700000000.000001" {
		t.Errorf("standalone.ID = %q, want %q", standalone.ID, "1700000000.000001")
	}
	if standalone.Author != "U111" {
		t.Errorf("standalone.Author = %q, want %q", standalone.Author, "U111")
	}
	if standalone.Tags["type"] != "message" {
		t.Errorf("standalone.Tags[type] = %q, want %q", standalone.Tags["type"], "message")
	}
	if standalone.Date.IsZero() {
		t.Error("standalone.Date should not be zero")
	}

	// Verify thread.
	thread := artifacts[1]
	if thread.Source != "slack" {
		t.Errorf("thread.Source = %q, want %q", thread.Source, "slack")
	}
	if thread.ID != "1700000001.000002" {
		t.Errorf("thread.ID = %q, want %q", thread.ID, "1700000001.000002")
	}
	if thread.Tags["type"] != "thread" {
		t.Errorf("thread.Tags[type] = %q, want %q", thread.Tags["type"], "thread")
	}
	if thread.Author != "U222" {
		t.Errorf("thread.Author = %q, want %q", thread.Author, "U222")
	}
	// Thread body should contain all participants' messages.
	if !containsSubstring(thread.Body, "adapter pattern") {
		t.Errorf("thread.Body missing reply content, got: %q", thread.Body)
	}
	if !containsSubstring(thread.Body, "Agreed") {
		t.Errorf("thread.Body missing second reply, got: %q", thread.Body)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && contains(s, sub))
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
