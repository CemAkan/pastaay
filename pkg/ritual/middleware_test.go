package ritual

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
)

// TestMiddleware_NoMatch ensures that requests not matching the target pass through normally.
func TestMiddleware_NoMatch(t *testing.T) {
	// 1. Setup config with a target that WON'T match our request
	cfg := &config.PastaayConfig{
		Policies: []config.Policy{
			{Target: "/api/v1/other", Type: "http", ErrorChance: 1.0}, // 100% error chance, but wrong path
		},
	}
	manager := config.NewManager(cfg)

	// 2. Create a dummy next handler that always returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 3. Wrap it with our middleware
	handlerToTest := Middleware(manager)(nextHandler)

	// 4. Create a mock request and a recorder to capture the response
	req := httptest.NewRequest("GET", "/api/v1/shorten", nil)
	recorder := httptest.NewRecorder()

	// 5. Serve the request
	handlerToTest.ServeHTTP(recorder, req)

	// 6. Assertions: We expect 200 OK because the path didn't match the policy
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, recorder.Code)
	}
}

// TestMiddleware_ErrorInjected ensures that requests matching the target get the injected error.
func TestMiddleware_ErrorInjected(t *testing.T) {
	// 1. Setup config with 100% error chance for our exact target
	cfg := &config.PastaayConfig{
		Policies: []config.Policy{
			{Target: "/api/v1/shorten", Type: "http", ErrorChance: 1.0},
		},
	}
	manager := config.NewManager(cfg)

	// 2. Dummy handler (should never be reached because middleware intercepts it)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := Middleware(manager)(nextHandler)

	req := httptest.NewRequest("GET", "/api/v1/shorten", nil)
	recorder := httptest.NewRecorder()

	handlerToTest.ServeHTTP(recorder, req)

	// 3. Assertions: We expect 500 Internal Server Error due to chaos injection
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}
