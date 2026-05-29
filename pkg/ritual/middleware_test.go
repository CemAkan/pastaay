package ritual

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
)

func TestMatchHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Test-User", "true")
	req.Header.Set("X-Device", "ios")

	tests := []struct {
		name     string
		required map[string]string
		expected bool
	}{
		{"No requirements should match", nil, true},
		{"Single matching requirement", map[string]string{"X-Test-User": "true"}, true},
		{"Mismatching requirement", map[string]string{"X-Test-User": "false"}, false},
		{"Missing required header", map[string]string{"X-Version": "v2"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchHeaders(req, tt.required); got != tt.expected {
				t.Errorf("matchHeaders() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name       string
		reqPath    string
		targetPath string
		expected   bool
	}{
		{"Exact match", "/api/v1/users", "/api/v1/users", true},
		{"Mismatch", "/api/v1/users", "/api/v2/users", false},
		{"Wildcard match exact", "/api/v1/users", "/api/v1/*", true},
		{"Wildcard match deep", "/api/v1/users/123/profile", "/api/v1/*", true},
		{"Wildcard mismatch", "/api/v2/users", "/api/v1/*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &config.Policy{Target: tt.targetPath}
			if strings.HasSuffix(tt.targetPath, "*") {
				p.IsWildcard = true
				p.WildcardPrefix = strings.ToUpper(tt.targetPath[:len(tt.targetPath)-1])
			}

			if got := matchPath(tt.reqPath, p); got != tt.expected {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.reqPath, tt.targetPath, got, tt.expected)
			}
		})
	}
}

func TestMiddleware_ErrorInjection(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{
			{
				Type:        "http",
				Target:      "/api/fail",
				ErrorChance: 1.0,
				ErrorCode:   http.StatusTooManyRequests,
				ErrorBody:   `{"error": "rate limited"}`,
			},
		},
	})

	middlewareFunc := Middleware(mgr)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	handler := middlewareFunc(nextHandler)

	reqFail := httptest.NewRequest(http.MethodGet, "/api/fail", nil)
	rrFail := httptest.NewRecorder()
	handler.ServeHTTP(rrFail, reqFail)

	if rrFail.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rrFail.Code)
	}
	if rrFail.Body.String() != `{"error": "rate limited"}` {
		t.Errorf("Expected body %q, got %q", `{"error": "rate limited"}`, rrFail.Body.String())
	}

	reqPass := httptest.NewRequest(http.MethodGet, "/api/success", nil)
	rrPass := httptest.NewRecorder()
	handler.ServeHTTP(rrPass, reqPass)

	if rrPass.Code != http.StatusOK {
		t.Errorf("Expected status %d for bypassed route, got %d", http.StatusOK, rrPass.Code)
	}
}
func TestMiddleware_LatencyOrError_NeverBoth(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{{
			Type:            "http",
			Target:          "/x",
			LatencyChance:   1.0,
			ErrorChance:     1.0,
			LatencyDuration: 5 * time.Millisecond,
			ErrorCode:       418,
			ErrorBody:       `{"err":"teapot"}`,
		}},
	})

	var nextCalled atomic.Bool
	mw := Middleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))

	// Tie-break
	for i := 0; i < 64; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rr := httptest.NewRecorder()
		start := time.Now()
		mw.ServeHTTP(rr, req)
		elapsed := time.Since(start)
		if rr.Code != http.StatusTeapot {
			t.Fatalf("iteration %d: tie-break must be error (418), got %d", i, rr.Code)
		}
		// Error path must NOT block on the latency timer.
		if elapsed > 4*time.Millisecond {
			t.Fatalf("iteration %d: error path took %v — latency timer leaked", i, elapsed)
		}
		if nextCalled.Load() {
			t.Fatalf("error path must short-circuit, next handler must not run")
		}
		nextCalled.Store(false)
	}
}

func TestMiddleware_LatencyPath_NoGoroutineLeak(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{{
			Type:            "http",
			Target:          "/x",
			LatencyChance:   1.0,
			LatencyDuration: 1 * time.Millisecond,
		}},
	})
	mw := Middleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Warm up.
	for i := 0; i < 100; i++ {
		mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	}
	runtime.GC()
	before := runtime.NumGoroutine()

	for i := 0; i < 4000; i++ {
		mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	}
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Fatalf("goroutine leak detected: before=%d after=%d", before, after)
	}
}

func TestMiddleware_ClientDisconnectDuringLatency(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{{
			Type:            "http",
			Target:          "/slow",
			LatencyChance:   1.0,
			LatencyDuration: 30 * time.Minute,
		}},
	})
	mw := Middleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/slow", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		mw.ServeHTTP(rr, req)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("middleware did not unblock on context cancel")
	}
	if rr.Code != 499 {
		t.Errorf("expected 499 (client closed), got %d", rr.Code)
	}
}

func TestMatchPath_WildcardEdgeCases(t *testing.T) {
	tests := []struct {
		req    string
		target string
		want   bool
	}{
		{"/api", "/api/*", false}, // slash boundary required
		{"/api/", "/api/*", true},
		{"/api/foo", "/api/*", true},
		{"/api/foo/bar", "/api/*", true},
		{"/apix", "/api/*", false},
		{"/api", "/api*", true}, // bare-prefix wildcard
		{"/api/foo", "/api*", true},
		{"/apix", "/api*", false}, // mid-token NOT matched
		{"/", "all", true},
		{"/v2", "/v1/*", false},
	}
	for _, tc := range tests {
		p := &config.Policy{Target: tc.target}
		if len(tc.target) > 0 && tc.target[len(tc.target)-1] == '*' {
			p.IsWildcard = true
			// emulate manager.go normalization
			p.WildcardPrefix = ""
			for _, c := range tc.target[:len(tc.target)-1] {
				if c >= 'a' && c <= 'z' {
					c -= 32
				}
				p.WildcardPrefix += string(c)
			}
		}
		if got := matchPath(tc.req, p); got != tc.want {
			t.Errorf("matchPath(%q, target=%q) = %v, want %v", tc.req, tc.target, got, tc.want)
		}
	}
}

func BenchmarkMiddleware_NoMatchPath(b *testing.B) {
	mgr := config.NewManager(&config.PastaayConfig{
		Version: 1,
		Policies: []config.Policy{
			{Type: "http", Target: "/none1", ErrorChance: 1.0},
			{Type: "http", Target: "/none2", ErrorChance: 1.0},
			{Type: "http", Target: "/none3", ErrorChance: 1.0},
		},
	})
	mw := Middleware(mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/passthrough", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
	}
}
