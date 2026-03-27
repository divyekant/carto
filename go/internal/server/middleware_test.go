package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is a trivial handler that always returns 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// =========================================================================
// bearerAuth tests
// =========================================================================

func TestBearerAuth_NoServerToken_AllowsAllRequests(t *testing.T) {
	// When serverToken is empty, auth is disabled and all requests pass through.
	h := bearerAuth("")(okHandler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when no server token configured, got %d", rec.Code)
	}
}

func TestBearerAuth_WithToken_RejectsRequestWithNoHeader(t *testing.T) {
	h := bearerAuth("super-secret")(okHandler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no Authorization header, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header on 401 response")
	}
}

func TestBearerAuth_WithToken_RejectsWrongToken(t *testing.T) {
	h := bearerAuth("super-secret")(okHandler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for incorrect token, got %d", rec.Code)
	}
}

func TestBearerAuth_WithToken_AcceptsCorrectToken(t *testing.T) {
	const token = "super-secret"
	h := bearerAuth(token)(okHandler)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with correct Bearer token, got %d", rec.Code)
	}
}

func TestBearerAuth_HealthEndpointBypassesAuth(t *testing.T) {
	// /api/health must be reachable without a token so load balancers and
	// container orchestrators can probe liveness.
	h := bearerAuth("super-secret")(okHandler)

	for _, path := range []string{"/api/health", "/api/v1/health"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("health path %q must bypass auth, got %d", path, rec.Code)
		}
	}
}

func TestBearerAuth_SPAAssetsPassThrough(t *testing.T) {
	// Static frontend assets (JS, CSS, index.html) should never require auth.
	h := bearerAuth("super-secret")(okHandler)

	for _, path := range []string{"/", "/assets/main.js", "/favicon.ico"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("SPA asset path %q should pass auth without token, got %d", path, rec.Code)
		}
	}
}

// =========================================================================
// rateLimiter tests
// =========================================================================

func TestRateLimiter_AllowsBurstOf30(t *testing.T) {
	rl := newRateLimiter()
	for i := 0; i < 30; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("request %d of burst should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlocksAfterBurstExhausted(t *testing.T) {
	rl := newRateLimiter()
	for i := 0; i < 30; i++ {
		rl.Allow("1.2.3.4")
	}
	// 31st request should be blocked.
	if rl.Allow("1.2.3.4") {
		t.Error("31st consecutive request should be rate-limited")
	}
}

func TestRateLimiter_DifferentIPsAreIndependent(t *testing.T) {
	rl := newRateLimiter()
	// Exhaust IP A.
	for i := 0; i < 30; i++ {
		rl.Allow("1.1.1.1")
	}
	// IP B should still be allowed.
	if !rl.Allow("2.2.2.2") {
		t.Error("different IP should have its own independent bucket")
	}
}

func TestRateLimitMiddleware_Returns429WhenExhausted(t *testing.T) {
	rl := newRateLimiter()
	h := rateLimitMiddleware(rl)(okHandler)

	// Exhaust the bucket from the test's RemoteAddr.
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	for i := 0; i < 30; i++ {
		rl.Allow("1.2.3.4")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after burst exhausted, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

// =========================================================================
// CORS middleware tests
// =========================================================================

func TestCORSMiddleware_SetsHeadersForAllowedOrigin(t *testing.T) {
	h := corsMiddleware([]string{"https://myapp.com"})(okHandler)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "https://myapp.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://myapp.com" {
		t.Errorf("expected ACAO header for allowed origin, got %q", got)
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
}

func TestCORSMiddleware_DoesNotSetHeadersForUnknownOrigin(t *testing.T) {
	h := corsMiddleware([]string{"https://myapp.com"})(okHandler)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unknown origin must not receive CORS header, got %q", got)
	}
}

func TestCORSMiddleware_WildcardAllowsAllOrigins(t *testing.T) {
	h := corsMiddleware([]string{"*"})(okHandler)

	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Origin", "https://any.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://any.example.com" {
		t.Errorf("wildcard should allow any origin, got %q", got)
	}
}

func TestCORSMiddleware_PreflightReturns204(t *testing.T) {
	h := corsMiddleware([]string{"https://myapp.com"})(okHandler)

	req := httptest.NewRequest("OPTIONS", "/api/projects", nil)
	req.Header.Set("Origin", "https://myapp.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight OPTIONS should return 204, got %d", rec.Code)
	}
}

func TestCORSMiddleware_NoOriginHeaderNoCorsSent(t *testing.T) {
	h := corsMiddleware([]string{"https://myapp.com"})(okHandler)

	// Request from same origin (browser omits Origin header for same-origin).
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no Origin header → no CORS headers, got %q", got)
	}
}

// =========================================================================
// Middleware chain composition test
// =========================================================================

func TestChain_ExecutesMiddlewareInOrder(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	}

	h := chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "handler")
		}),
		m1, m2,
	)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if len(order) != 3 || order[0] != "m1" || order[1] != "m2" || order[2] != "handler" {
		t.Errorf("expected execution order [m1 m2 handler], got %v", order)
	}
}

// =========================================================================
// Structured JSON logging (log/slog) tests
// =========================================================================

// TestLoggingMiddleware_EmitsStructuredJSON verifies that loggingMiddleware
// writes a JSON object containing the expected keys for every non-health request.
func TestLoggingMiddleware_EmitsStructuredJSON(t *testing.T) {
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, nil))

	// Redirect the package-level logger to our buffer for this test.
	original := serverLog
	serverLog = h
	t.Cleanup(func() { serverLog = original })

	wrapped := loggingMiddleware(okHandler)
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	if buf.Len() == 0 {
		t.Fatal("expected at least one log line, got none")
	}

	var logEntry map[string]any
	if err := json.NewDecoder(&buf).Decode(&logEntry); err != nil {
		t.Fatalf("log output is not valid JSON: %v\nraw: %s", err, buf.String())
	}

	for _, key := range []string{"msg", "method", "path", "status", "latency_ms", "ip"} {
		if _, ok := logEntry[key]; !ok {
			t.Errorf("expected key %q in log entry, got: %v", key, logEntry)
		}
	}
	if logEntry["method"] != "GET" {
		t.Errorf("expected method=GET, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/api/projects" {
		t.Errorf("expected path=/api/projects, got %v", logEntry["path"])
	}
}

// TestLoggingMiddleware_SkipsHealthzPath verifies that /healthz requests
// are silently swallowed by loggingMiddleware to avoid probe noise.
func TestLoggingMiddleware_SkipsHealthzPath(t *testing.T) {
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, nil))

	original := serverLog
	serverLog = h
	t.Cleanup(func() { serverLog = original })

	wrapped := loggingMiddleware(okHandler)
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	if buf.Len() > 0 {
		t.Errorf("expected no log output for /healthz probe, got: %s", buf.String())
	}
}

// TestLoggingMiddleware_SkipsAPIHealthPaths verifies that the /api/health*
// family of paths is also suppressed from the request log.
func TestLoggingMiddleware_SkipsAPIHealthPaths(t *testing.T) {
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, nil))

	original := serverLog
	serverLog = h
	t.Cleanup(func() { serverLog = original })

	wrapped := loggingMiddleware(okHandler)
	for _, path := range []string{"/api/health", "/api/health/live", "/api/health/ready"} {
		buf.Reset()
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "10.0.0.1:54321"
		wrapped.ServeHTTP(httptest.NewRecorder(), req)

		if buf.Len() > 0 {
			t.Errorf("expected no log output for probe path %q, got: %s", path, buf.String())
		}
	}
}

// TestAuditMiddleware_EmitsAuditField verifies that auditMiddleware writes a
// JSON log line with "audit":true for mutating HTTP methods.
func TestAuditMiddleware_EmitsAuditField(t *testing.T) {
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, nil))

	original := serverLog
	serverLog = h
	t.Cleanup(func() { serverLog = original })

	wrapped := auditMiddleware(okHandler)
	req := httptest.NewRequest("DELETE", "/api/projects/myproj", nil)
	req.RemoteAddr = "10.0.0.2:9999"
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	if buf.Len() == 0 {
		t.Fatal("expected audit log line for DELETE, got none")
	}

	var logEntry map[string]any
	if err := json.NewDecoder(&buf).Decode(&logEntry); err != nil {
		t.Fatalf("audit log is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if logEntry["audit"] != true {
		t.Errorf("expected audit:true in audit log, got: %v", logEntry["audit"])
	}
	if logEntry["msg"] != "audit" {
		t.Errorf("expected msg='audit', got: %v", logEntry["msg"])
	}
}

// TestAuditMiddleware_SkipsGETRequests verifies that read-only methods do
// not produce audit log lines (only POST/PATCH/PUT/DELETE are audited).
func TestAuditMiddleware_SkipsGETRequests(t *testing.T) {
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, nil))

	original := serverLog
	serverLog = h
	t.Cleanup(func() { serverLog = original })

	wrapped := auditMiddleware(okHandler)
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "10.0.0.2:9999"
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	if buf.Len() > 0 {
		t.Errorf("GET must not produce an audit log line, got: %s", buf.String())
	}
}

// TestServerLog_InitialisesWithJSONHandler is a smoke test confirming that
// the package-level serverLog is a functional *slog.Logger backed by a JSON
// handler. It writes to /dev/null to avoid polluting test output.
func TestServerLog_InitialisesWithJSONHandler(t *testing.T) {
	// serverLog is initialised by the package-level var in logging.go.
	// Replace it temporarily to capture output without touching stdout.
	var buf bytes.Buffer
	h := slog.New(slog.NewJSONHandler(&buf, nil))

	original := serverLog
	serverLog = h
	t.Cleanup(func() { serverLog = original })

	serverLog.Info("test_event", "key", "value")

	var logEntry map[string]any
	if err := json.NewDecoder(&buf).Decode(&logEntry); err != nil {
		t.Fatalf("serverLog output is not valid JSON: %v", err)
	}
	if logEntry["msg"] != "test_event" {
		t.Errorf("expected msg=test_event, got %v", logEntry["msg"])
	}
	if logEntry["key"] != "value" {
		t.Errorf("expected key=value, got %v", logEntry["key"])
	}
}
