package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropic/indexer/internal/atoms"
	"github.com/anthropic/indexer/internal/llm"
)

// mockLLM implements LLMClient for testing.
type mockLLM struct {
	mu        sync.Mutex
	responses map[string]string // prompt substring -> JSON response
	calls     int
	tiers     []llm.Tier
}

func (m *mockLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.tiers = append(m.tiers, tier)

	for substr, resp := range m.responses {
		if strings.Contains(prompt, substr) {
			return json.RawMessage(resp), nil
		}
	}

	// If no substring matches, return the first response found (fallback).
	for _, resp := range m.responses {
		return json.RawMessage(resp), nil
	}

	return nil, fmt.Errorf("mockLLM: no response configured for prompt")
}

// errorLLM returns an error for specific call indices (0-based).
type errorLLM struct {
	mu        sync.Mutex
	calls     int
	errorOn   map[int]bool
	validResp string
	tiers     []llm.Tier
}

func (m *errorLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	idx := m.calls
	m.calls++
	m.tiers = append(m.tiers, tier)
	shouldError := m.errorOn[idx]
	m.mu.Unlock()

	if shouldError {
		return nil, fmt.Errorf("simulated LLM error")
	}
	return json.RawMessage(m.validResp), nil
}

const validModuleResponse = `{
	"module_name": "auth",
	"wiring": [
		{"from": "LoginHandler", "to": "UserStore", "reason": "reads user credentials"},
		{"from": "SessionManager", "to": "TokenService", "reason": "generates JWT tokens"}
	],
	"zones": [
		{
			"name": "authentication",
			"intent": "Handles user login and credential verification",
			"files": ["auth/login.go", "auth/credentials.go"]
		},
		{
			"name": "session",
			"intent": "Manages user sessions and tokens",
			"files": ["auth/session.go", "auth/token.go"]
		}
	],
	"module_intent": "Provides authentication and session management for the application. Handles login, credential verification, and JWT token lifecycle."
}`

const validSynthesisResponse = `{
	"blueprint": "The system is a web application with a layered architecture. The auth module handles user identity, while the api module exposes HTTP endpoints. The storage module provides persistence via PostgreSQL.",
	"patterns": [
		"Dependency injection via constructor functions",
		"Interface-based abstractions for testability",
		"Horizontal scaling through stateless request handlers"
	]
}`

func sampleModuleInput(name string) ModuleInput {
	return ModuleInput{
		Name: name,
		Path: "internal/" + name,
		Atoms: []*atoms.Atom{
			{
				Name:     "HandleLogin",
				Kind:     "function",
				FilePath: "internal/" + name + "/login.go",
				Summary:  "Handles user login requests",
				Imports:  []string{"net/http", "encoding/json"},
				Exports:  []string{"HandleLogin"},
			},
			{
				Name:     "UserStore",
				Kind:     "struct",
				FilePath: "internal/" + name + "/store.go",
				Summary:  "Stores and retrieves user records",
				Imports:  []string{"database/sql"},
				Exports:  []string{"UserStore", "NewUserStore"},
			},
		},
	}
}

func TestAnalyzeModule(t *testing.T) {
	mock := &mockLLM{
		responses: map[string]string{
			"auth": validModuleResponse,
		},
	}
	da := NewDeepAnalyzer(mock)

	input := sampleModuleInput("auth")
	result, err := da.AnalyzeModule(input)
	if err != nil {
		t.Fatalf("AnalyzeModule returned error: %v", err)
	}

	// Verify module name.
	if result.ModuleName != "auth" {
		t.Errorf("ModuleName: got %q, want %q", result.ModuleName, "auth")
	}

	// Verify wiring.
	if len(result.Wiring) != 2 {
		t.Fatalf("Wiring: got %d entries, want 2", len(result.Wiring))
	}
	if result.Wiring[0].From != "LoginHandler" {
		t.Errorf("Wiring[0].From: got %q, want %q", result.Wiring[0].From, "LoginHandler")
	}
	if result.Wiring[0].To != "UserStore" {
		t.Errorf("Wiring[0].To: got %q, want %q", result.Wiring[0].To, "UserStore")
	}
	if result.Wiring[0].Reason != "reads user credentials" {
		t.Errorf("Wiring[0].Reason: got %q, want %q", result.Wiring[0].Reason, "reads user credentials")
	}
	if result.Wiring[1].From != "SessionManager" {
		t.Errorf("Wiring[1].From: got %q, want %q", result.Wiring[1].From, "SessionManager")
	}

	// Verify zones.
	if len(result.Zones) != 2 {
		t.Fatalf("Zones: got %d entries, want 2", len(result.Zones))
	}
	if result.Zones[0].Name != "authentication" {
		t.Errorf("Zones[0].Name: got %q, want %q", result.Zones[0].Name, "authentication")
	}
	if result.Zones[0].Intent != "Handles user login and credential verification" {
		t.Errorf("Zones[0].Intent: got %q, want %q", result.Zones[0].Intent, "Handles user login and credential verification")
	}
	if len(result.Zones[0].Files) != 2 {
		t.Fatalf("Zones[0].Files: got %d, want 2", len(result.Zones[0].Files))
	}
	if result.Zones[0].Files[0] != "auth/login.go" {
		t.Errorf("Zones[0].Files[0]: got %q, want %q", result.Zones[0].Files[0], "auth/login.go")
	}
	if result.Zones[1].Name != "session" {
		t.Errorf("Zones[1].Name: got %q, want %q", result.Zones[1].Name, "session")
	}

	// Verify module intent.
	if result.ModuleIntent == "" {
		t.Error("ModuleIntent should not be empty")
	}
	if !strings.Contains(result.ModuleIntent, "authentication") {
		t.Errorf("ModuleIntent should mention authentication, got: %q", result.ModuleIntent)
	}

	// Verify mock was called exactly once.
	mock.mu.Lock()
	calls := mock.calls
	mock.mu.Unlock()
	if calls != 1 {
		t.Errorf("LLM calls: got %d, want 1", calls)
	}
}

func TestAnalyzeModule_UsesOpus(t *testing.T) {
	mock := &mockLLM{
		responses: map[string]string{
			"auth": validModuleResponse,
		},
	}
	da := NewDeepAnalyzer(mock)

	input := sampleModuleInput("auth")
	_, err := da.AnalyzeModule(input)
	if err != nil {
		t.Fatalf("AnalyzeModule returned error: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()

	if len(mock.tiers) != 1 {
		t.Fatalf("expected 1 tier recorded, got %d", len(mock.tiers))
	}
	if mock.tiers[0] != llm.TierOpus {
		t.Errorf("tier: got %q, want %q", mock.tiers[0], llm.TierOpus)
	}
}

func TestSynthesizeSystem(t *testing.T) {
	mock := &mockLLM{
		responses: map[string]string{
			"Synthesize": validSynthesisResponse,
		},
	}
	da := NewDeepAnalyzer(mock)

	modules := []ModuleAnalysis{
		{
			ModuleName:   "auth",
			ModuleIntent: "Handles authentication",
			Wiring:       []Dependency{{From: "A", To: "B", Reason: "test"}},
			Zones:        []Zone{{Name: "login", Intent: "user login", Files: []string{"auth/login.go"}}},
		},
		{
			ModuleName:   "api",
			ModuleIntent: "Exposes HTTP endpoints",
			Wiring:       []Dependency{{From: "C", To: "D", Reason: "routing"}},
			Zones:        []Zone{{Name: "handlers", Intent: "request handling", Files: []string{"api/handler.go"}}},
		},
	}

	result, err := da.SynthesizeSystem(modules)
	if err != nil {
		t.Fatalf("SynthesizeSystem returned error: %v", err)
	}

	// Verify blueprint.
	if result.Blueprint == "" {
		t.Error("Blueprint should not be empty")
	}
	if !strings.Contains(result.Blueprint, "web application") {
		t.Errorf("Blueprint should describe the system, got: %q", result.Blueprint)
	}

	// Verify patterns.
	if len(result.Patterns) != 3 {
		t.Fatalf("Patterns: got %d, want 3", len(result.Patterns))
	}
	if !strings.Contains(result.Patterns[0], "Dependency injection") {
		t.Errorf("Patterns[0]: got %q, want substring %q", result.Patterns[0], "Dependency injection")
	}
	if !strings.Contains(result.Patterns[1], "Interface-based") {
		t.Errorf("Patterns[1]: got %q, want substring %q", result.Patterns[1], "Interface-based")
	}

	// Verify the LLM was called with Opus tier.
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d", len(mock.tiers))
	}
	if mock.tiers[0] != llm.TierOpus {
		t.Errorf("tier: got %q, want %q", mock.tiers[0], llm.TierOpus)
	}
}

func TestAnalyzeModules_Parallel(t *testing.T) {
	mock := &mockLLM{
		responses: map[string]string{
			"Analyze the module": validModuleResponse,
		},
	}
	da := NewDeepAnalyzer(mock)

	modules := []ModuleInput{
		sampleModuleInput("auth"),
		sampleModuleInput("api"),
		sampleModuleInput("storage"),
	}

	var progressCalls atomic.Int32
	var lastDone, lastTotal atomic.Int32

	results, err := da.AnalyzeModules(modules, 2, func(done, total int) {
		progressCalls.Add(1)
		lastDone.Store(int32(done))
		lastTotal.Store(int32(total))
	})
	if err != nil {
		t.Fatalf("AnalyzeModules returned error: %v", err)
	}

	// All 3 modules should produce results.
	if len(results) != 3 {
		t.Errorf("got %d results, want 3", len(results))
	}

	// Progress should have been called 3 times (once per module).
	if pc := progressCalls.Load(); pc != 3 {
		t.Errorf("progress called %d times, want 3", pc)
	}

	// Final total should be 3.
	if lt := lastTotal.Load(); lt != 3 {
		t.Errorf("last total: got %d, want 3", lt)
	}

	// LLM should have been called 3 times.
	mock.mu.Lock()
	calls := mock.calls
	mock.mu.Unlock()
	if calls != 3 {
		t.Errorf("LLM calls: got %d, want 3", calls)
	}
}

func TestAnalyzeModule_EmptyAtoms(t *testing.T) {
	mock := &mockLLM{
		responses: map[string]string{
			"Analyze the module": validModuleResponse,
		},
	}
	da := NewDeepAnalyzer(mock)

	input := ModuleInput{
		Name: "empty",
		Path: "internal/empty",
		// Atoms, History, and Signals are all nil/empty.
	}

	result, err := da.AnalyzeModule(input)
	if err != nil {
		t.Fatalf("AnalyzeModule with empty atoms returned error: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil for empty input")
	}

	// The module name should still be set.
	if result.ModuleName != "auth" && result.ModuleName != "empty" {
		// The mock response has module_name "auth"; the code sets it from input
		// only if the LLM omits it. Since our mock returns "auth", that's fine.
		t.Errorf("ModuleName: got %q", result.ModuleName)
	}

	// Verify the prompt was built without panicking, and the LLM was called.
	mock.mu.Lock()
	calls := mock.calls
	mock.mu.Unlock()
	if calls != 1 {
		t.Errorf("LLM calls: got %d, want 1", calls)
	}
}

func TestAnalyzeModules_SkipsErrors(t *testing.T) {
	// Error on the second call (index 1).
	mock := &errorLLM{
		errorOn:   map[int]bool{1: true},
		validResp: validModuleResponse,
	}
	da := NewDeepAnalyzer(mock)

	modules := []ModuleInput{
		sampleModuleInput("auth"),
		sampleModuleInput("api"),
		sampleModuleInput("storage"),
	}

	var progressCalls atomic.Int32

	// Use maxWorkers=1 so call order is deterministic.
	results, err := da.AnalyzeModules(modules, 1, func(done, total int) {
		progressCalls.Add(1)
	})

	// Should return an error indicating one module failed.
	if err == nil {
		t.Fatal("expected error from AnalyzeModules when one module fails")
	}
	if !strings.Contains(err.Error(), "1 module(s) failed") {
		t.Errorf("error should mention 1 module failed, got: %v", err)
	}

	// 1 of 3 modules errored, so we should get 2 results.
	if len(results) != 2 {
		t.Errorf("got %d results, want 2 (1 error skipped)", len(results))
	}

	// Progress should still be called for all 3 modules.
	if pc := progressCalls.Load(); pc != 3 {
		t.Errorf("progress called %d times, want 3", pc)
	}
}
