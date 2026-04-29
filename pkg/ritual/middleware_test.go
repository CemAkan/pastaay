package ritual

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
)

// Header Matching
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

// Path and Wildcard Matching
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
			if got := matchPath(tt.reqPath, tt.targetPath); got != tt.expected {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.reqPath, tt.targetPath, got, tt.expected)
			}
		})
	}
}

// --- 3. NEW TEST: Middleware Chaos Injection ---
func TestMiddleware_ErrorInjection(t *testing.T) {
	// Setup: Create a dynamic Manager and a mock HTTP policy
	mgr := config.NewManager(&config.PastaayConfig{
		Policies: []config.Policy{
			{
				Type:        "http",
				Target:      "/api/fail",
				ErrorChance: 1.0,                        // 100% chance to throw an error
				ErrorCode:   http.StatusTooManyRequests, // Returns HTTP 429
				ErrorBody:   `{"error": "rate limited"}`,
			},
		},
	})

	// Wrap the middleware
	middlewareFunc := Middleware(mgr)

	// A mock endpoint that normally works (returns 200 OK)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap the handler with the chaos middleware
	handler := middlewareFunc(nextHandler)

	// SCENARIO A: Request to the targeted path (Should be sabotaged)
	reqFail := httptest.NewRequest(http.MethodGet, "/api/fail", nil)
	rrFail := httptest.NewRecorder()
	handler.ServeHTTP(rrFail, reqFail)

	if rrFail.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rrFail.Code)
	}
	if rrFail.Body.String() != `{"error": "rate limited"}` {
		t.Errorf("Expected body %q, got %q", `{"error": "rate limited"}`, rrFail.Body.String())
	}

	// SCENARIO B: Request to an untargeted path (Should work normally)
	reqPass := httptest.NewRequest(http.MethodGet, "/api/success", nil)
	rrPass := httptest.NewRecorder()
	handler.ServeHTTP(rrPass, reqPass)

	if rrPass.Code != http.StatusOK {
		t.Errorf("Expected status %d for bypassed route, got %d", http.StatusOK, rrPass.Code)
	}
}
