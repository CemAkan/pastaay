package web

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/CemAkan/pastaay/pkg/config"
)

func TestRequireConsoleToken_FailClosed(t *testing.T) {
	t.Setenv("PASTAAY_DEV_ALLOW_NO_TOKEN", "")
	called := false
	h := requireConsoleToken("", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 fail-closed, got %d", rr.Code)
	}
	if called {
		t.Fatal("handler must not run when token is unconfigured")
	}
}

func TestRequireConsoleToken_DevAllowNoToken(t *testing.T) {
	t.Setenv("PASTAAY_DEV_ALLOW_NO_TOKEN", "1")
	called := false
	h := requireConsoleToken("", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rr.Code != http.StatusOK || !called {
		t.Fatalf("dev override must allow access; code=%d called=%v", rr.Code, called)
	}
}

func TestRequireConsoleToken_ConstantTimeReject(t *testing.T) {
	h := requireConsoleToken("expected-secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Pastaay-Token", "wrong")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireConsoleToken_CSRF_PostFormDenied(t *testing.T) {
	h := requireConsoleToken("token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("a=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Pastaay-Token", "token")
	req.Header.Set("Origin", "https://evil.example.com")
	req.Host = "console.example.com"
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 CSRF, got %d", rr.Code)
	}
}

func TestRequireConsoleToken_CSRF_PostJSONAllowed(t *testing.T) {
	h := requireConsoleToken("token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pastaay-Token", "token")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("application/json POST must pass CSRF gate, got %d", rr.Code)
	}
}

func TestHandleProbe_RejectsInternalLiteral(t *testing.T) {
	body := strings.NewReader(`{"url":"http://127.0.0.1/admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/probe", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleProbe(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("probe handler should return JSON even on refusal, got code %d", rr.Code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	errStr, _ := out["error"].(string)
	if !strings.Contains(errStr, "internal") && !strings.Contains(errStr, "refused") {
		t.Fatalf("expected internal-IP refusal, got %v", out)
	}
}

func TestHandleProbe_RejectsRFC1918(t *testing.T) {
	body := strings.NewReader(`{"url":"http://10.0.0.5/x"}`)
	req := httptest.NewRequest(http.MethodPost, "/probe", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleProbe(rr, req)
	var out map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if errStr, _ := out["error"].(string); !strings.Contains(errStr, "internal") {
		t.Fatalf("expected internal refusal, got %v", out)
	}
}

func TestHandleProbe_RejectsLinkLocal(t *testing.T) {
	body := strings.NewReader(`{"url":"http://169.254.169.254/latest/meta-data/"}`)
	req := httptest.NewRequest(http.MethodPost, "/probe", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleProbe(rr, req)
	if !strings.Contains(rr.Body.String(), "internal") {
		t.Fatalf("EC2 metadata service IP must be rejected (got %s)", rr.Body.String())
	}
}

func TestHandleProbe_RejectsBadSchemes(t *testing.T) {
	for _, url := range []string{
		"file:///etc/passwd",
		"gopher://internal/x",
		"ftp://example.com",
		"",
	} {
		body := strings.NewReader(`{"url":"` + url + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/probe", body)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handleProbe(rr, req)
		if rr.Code != http.StatusBadRequest && !strings.Contains(rr.Body.String(), "error") && url != "" {
			t.Errorf("scheme %q should be rejected, body=%s", url, rr.Body.String())
		}
	}
}

func TestIsInternalIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true}, // CGNAT
		{"100.127.255.254", true},
		{"100.128.0.1", false}, // outside CGNAT
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::1", true},
		{"fe80::1", true},
		{"ff00::1", true},
		{"2001:4860:4860::8888", false}, // Google DNS v6
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if got := isInternalIP(ip); got != c.want {
			t.Errorf("isInternalIP(%s) = %v want %v", c.ip, got, c.want)
		}
	}
}

func TestHandleOracle_ServerSideKeyOnly(t *testing.T) {
	_ = os.Unsetenv("PASTAAY_OPENAI_KEY")
	body := strings.NewReader(`{"provider":"openai","prompt":"x","intensity":"low"}`)
	req := httptest.NewRequest(http.MethodPost, "/oracle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleOracle(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when provider not configured, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleOracle_RejectsBadProvider(t *testing.T) {
	body := strings.NewReader(`{"provider":"hacker","prompt":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/oracle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleOracle(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported provider, got %d", rr.Code)
	}
}

func TestHandleOracle_RejectsHugeBody(t *testing.T) {
	big := strings.Repeat("a", maxAPIRequestBytes+1024)
	body := strings.NewReader(`{"provider":"openai","prompt":"` + big + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/oracle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleOracle(rr, req)
	// decoder errors with 400.
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on oversized body, got %d", rr.Code)
	}
}

func TestRegisterHandlers_RespectsCancelledContext(t *testing.T) {
	mgr := config.NewManager(&config.PastaayConfig{Version: 1})
	mux := http.NewServeMux()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	RegisterHandlers(ctx, mux, mgr) // must not panic and block
}
