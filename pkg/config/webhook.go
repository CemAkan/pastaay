package config

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

func mustCompileMulti(pattern string) *regexp.Regexp {
	return regexp.MustCompile("(?m)" + pattern)
}

const maxWebhookPayloadBytes = 1 << 20 // 1MB Safety Cap

type WebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// WebhookHandler provides a memory-bounded, constant-time authenticated endpoint.
// Empty expectedToken refuses requests unless PASTAAY_DEV_ALLOW_NO_TOKEN=1.
func WebhookHandler(expectedToken string, reloadCallback func(*PastaayConfig)) http.HandlerFunc {
	devAllow := os.Getenv("PASTAAY_DEV_ALLOW_NO_TOKEN") == "1"
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
			return
		}

		defer r.Body.Close()

		// Auth gate — fail-closed by default.
		if expectedToken == "" {
			if !devAllow {
				writeJSONError(w, http.StatusServiceUnavailable, "engine token not configured", "")
				return
			}
			log.Printf("[Pastaay-Webhook] DEV: unauthenticated %s from %s", r.URL.Path, r.RemoteAddr)
		} else {
			reqToken := r.Header.Get("X-Pastaay-Token")
			if subtle.ConstantTimeCompare([]byte(reqToken), []byte(expectedToken)) != 1 {
				log.Printf("[Pastaay-Webhook] Blocked unauthorized request from %s", r.RemoteAddr)
				writeJSONError(w, http.StatusUnauthorized, "Invalid token", "")
				return
			}
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookPayloadBytes))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Payload read error or too large", "")
			return
		}

		// Reject YAML alias bombs before they reach the parser. yaml.v3 does
		// not enforce alias-expansion limits.
		if hasSuspiciousYAMLAlias(body) {
			writeJSONError(w, http.StatusBadRequest, "YAML aliases are not permitted", "")
			return
		}

		var newCfg PastaayConfig
		if err := yaml.Unmarshal(body, &newCfg); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid YAML/JSON syntax", err.Error())
			return
		}

		if err := newCfg.Validate(); err != nil {
			log.Printf("[Pastaay-Webhook] Rejected invalid config from %s: %v", r.RemoteAddr, err)
			writeJSONError(w, http.StatusUnprocessableEntity, "Policy validation failed", err.Error())
			return
		}

		reloadCallback(&newCfg)
		log.Printf("[Pastaay-Webhook] Engine memory updated successfully from %s", r.RemoteAddr)

		if err := json.NewEncoder(w).Encode(WebhookResponse{Status: "success", Message: "Applied"}); err != nil {
			log.Printf("[Pastaay-Webhook] success response encode failed: %v", err)
		}
	}
}

// hasSuspiciousYAMLAlias is a conservative pre-filter against billion-laughs (alias-bomb) DoS.
func hasSuspiciousYAMLAlias(body []byte) bool {
	s := stripYAMLStringLiterals(string(body))
	return reYAMLAnchor.MatchString(s) && reYAMLAlias.MatchString(s)
}

// stripYAMLStringLiterals removes single/double-quoted regions so that values
func stripYAMLStringLiterals(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\'' || c == '"' {
			quote := c
			i++

			for i < len(s) && s[i] != quote {
				if quote == '"' && s[i] == '\\' && i+1 < len(s) {
					i += 2
					continue
				}
				i++
			}
			if i < len(s) {
				i++ // consume closing quote
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// reYAMLAnchor matches an anchor declaration "&name" appearing after any of:
// start-of-line, colon-space, dash-space, comma, opening bracket, or opening
// brace. This covers block and flow styles.
var reYAMLAnchor = mustCompileMulti(`(^|[\s\:\,\-\[\{])&[A-Za-z0-9_\-]+`)

// reYAMLAlias matches an alias reference "*name" appearing in the same
// positions. Crucially this catches flow-style bombs like `[*a,*a,*a]`.
var reYAMLAlias = mustCompileMulti(`(^|[\s\:\,\-\[\{])\*[A-Za-z0-9_\-]+`)

func writeJSONError(w http.ResponseWriter, code int, msg, details string) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(WebhookResponse{Status: "error", Message: msg, Details: details}); err != nil {
		log.Printf("[Pastaay-Webhook] error response encode failed (code=%d): %v", code, err)
	}
}

func ExportHandler(mgr *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
			return
		}

		w.Header().Set("Content-Type", "application/yaml")
		cfg := mgr.GetRawConfig()

		if cfg == nil {
			if _, err := w.Write([]byte("version: 1\npolicies: []\n")); err != nil {
				log.Printf("[Pastaay-Webhook] export empty-cfg write failed: %v", err)
			}
			return
		}

		enc := yaml.NewEncoder(w)
		defer enc.Close()
		if err := enc.Encode(cfg); err != nil {
			log.Printf("[Pastaay-Webhook] export YAML encode failed: %v", err)
		}
	}
}
