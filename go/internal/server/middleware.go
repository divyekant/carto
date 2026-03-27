package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// =========================================================================
// Context key type
// =========================================================================

type contextKey int

const (
	ctxRequestID contextKey = iota
)

// =========================================================================
// Request-ID Middleware
// =========================================================================

// requestIDMiddleware assigns every request a unique X-Request-ID. If the
// client provides one it is forwarded (sanitised to max 64 chars). The ID is
// stored on the request context and echoed in the response header so callers
// can correlate log lines with API responses — a standard B2B observability
// practice.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			} else {
				id = "fallback"
			}
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromCtx extracts the request ID from a request context. Returns
// an empty string when absent (e.g. in unit tests that bypass the middleware).
func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

// =========================================================================
// Audit Middleware (mutating operations)
// =========================================================================

// auditMiddleware records every mutating HTTP request (POST, PATCH, PUT,
// DELETE) along with its outcome. Read-only and health-probe requests are
// excluded. Each line is structured JSON matching the loggingMiddleware
// schema so log aggregators (Datadog, CloudWatch, Loki) can parse both.
func auditMiddleware(next http.Handler) http.Handler {
	mutating := map[string]struct{}{
		http.MethodPost:   {},
		http.MethodPatch:  {},
		http.MethodPut:    {},
		http.MethodDelete: {},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, isMutating := mutating[r.Method]; !isMutating {
			next.ServeHTTP(w, r)
			return
		}

		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)

		requestID := RequestIDFromCtx(r.Context())
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)

		// "audit":true is a structured field that log aggregators can filter on.
		// slog's JSON handler adds "ts" and "level" automatically.
		serverLog.Info("audit",
			"audit", true,
			"method", r.Method,
			"path", r.URL.Path,
			"status", lrw.status,
			"request_id", requestID,
			"ip", ip,
		)
	})
}

// =========================================================================
// Auth Middleware
// =========================================================================

// bearerAuth returns middleware that enforces Bearer token authentication.
// When serverToken is empty, all requests are allowed (development mode).
// The /api/health path always bypasses authentication so container
// orchestrators and load balancers can probe liveness without credentials.
func bearerAuth(serverToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Empty token → auth disabled (dev/trusted-network mode).
			if serverToken == "" {
				next.ServeHTTP(w, r)
				return
			}

			// All health endpoints bypass auth so container orchestrators and
			// load-balancer probes can check liveness/readiness without credentials.
			p := r.URL.Path
			if p == "/api/health" || p == "/api/v1/health" ||
				p == "/api/health/live" || p == "/api/health/ready" {
				next.ServeHTTP(w, r)
				return
			}

			// Static SPA assets never require auth (index.html, JS, CSS).
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				w.Header().Set("WWW-Authenticate", `Bearer realm="carto"`)
				writeError(w, http.StatusUnauthorized, "Authorization header required")
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if token != serverToken {
				w.Header().Set("WWW-Authenticate", `Bearer realm="carto"`)
				writeError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// =========================================================================
// Logging Middleware
// =========================================================================

// loggingResponseWriter wraps http.ResponseWriter to capture the status code
// written by the downstream handler for structured logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	if !l.wrote {
		l.status = code
		l.wrote = true
	}
	l.ResponseWriter.WriteHeader(code)
}

func (l *loggingResponseWriter) Write(b []byte) (int, error) {
	if !l.wrote {
		l.status = http.StatusOK
		l.wrote = true
	}
	return l.ResponseWriter.Write(b)
}

// loggingMiddleware emits a structured JSON log line for every HTTP request,
// including method, path, response status code, latency in milliseconds, and
// the X-Request-ID for log correlation. Compatible with Datadog, CloudWatch,
// and Loki structured log parsers.
//
// It also increments the package-level requestCounter so /api/metrics can
// report total request throughput without additional overhead.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lrw, r)
		latency := time.Since(start).Milliseconds()

		// Always count requests for the /api/metrics total_requests counter.
		requestCounter.Add(1)

		// Omit high-frequency health-probe paths to reduce noise.
		// /healthz is the Kubernetes-standard root-level liveness path;
		// the /api/health/* family is the tiered API family.
		p := r.URL.Path
		if p == "/healthz" ||
			p == "/api/health" || p == "/api/v1/health" ||
			p == "/api/health/live" || p == "/api/health/ready" {
			return
		}

		requestID := RequestIDFromCtx(r.Context())
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)

		// slog's JSON handler adds "ts" and "level" automatically.
		serverLog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lrw.status,
			"latency_ms", latency,
			"ip", ip,
			"request_id", requestID,
		)
	})
}

// =========================================================================
// Rate Limiter
// =========================================================================

// bucket holds a token-bucket state for a single IP address.
type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// rateLimiter implements a per-IP token-bucket rate limiter using only stdlib.
// It is safe for concurrent use. Bucket state is lazily created and entries
// older than 10 minutes are evicted on each Allow call (amortised cleanup).
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string]*bucket)}
}

// Allow returns true if the given IP has remaining capacity in its token bucket.
// Default parameters: 60 requests/minute capacity, 10-request burst.
func (rl *rateLimiter) Allow(ip string) bool {
	const (
		ratePerSec = 5.0  // 300 req/min = 5 req/sec
		burst      = 30.0 // initial and max token count
	)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Lazy eviction: purge stale buckets to bound memory growth.
	for k, b := range rl.buckets {
		if now.Sub(b.lastSeen) > 10*time.Minute {
			delete(rl.buckets, k)
		}
	}

	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{tokens: burst, lastSeen: now}
		rl.buckets[ip] = b
	}

	// Replenish tokens based on elapsed time since last request.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens = min(burst, b.tokens+elapsed*ratePerSec)
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimitMiddleware wraps a handler with per-IP rate limiting. Requests that
// exceed the rate limit receive a 429 Too Many Requests response with a
// Retry-After header advising clients to wait 1 second before retrying.
func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr // fallback: use raw addr
			}
			if !rl.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded — retry after 1 second")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// =========================================================================
// CORS Middleware
// =========================================================================

// corsMiddleware sets Access-Control-Allow-* headers for listed origins.
// Pass ["*"] to allow all origins (not recommended in production).
// An empty allowedOrigins list disables CORS headers entirely (same-origin only).
// OPTIONS preflight requests receive a 204 No Content response.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	// Build a set for O(1) lookup.
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				// Check wildcard first, then exact match.
				if _, ok := allowed["*"]; ok {
					setCORSHeaders(w, origin)
				} else if _, ok := allowed[origin]; ok {
					setCORSHeaders(w, origin)
				}
			}

			// Handle preflight.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
	w.Header().Set("Access-Control-Max-Age", "3600")
	// Vary header ensures correct caching by CDNs when multiple origins are allowed.
	w.Header().Add("Vary", "Origin")
}

// =========================================================================
// Middleware Chain Builder
// =========================================================================

// chain applies a list of middleware functions to a handler in order.
// The first middleware in the slice is the outermost wrapper (first to execute).
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply in reverse so the first middleware in the list runs first.
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// min returns the lesser of two float64 values (backfills Go 1.20 builtin for older toolchains).
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
