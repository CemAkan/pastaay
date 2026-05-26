package config

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"gopkg.in/yaml.v3"
)

const maxWebhookPayloadBytes = 1 << 20 // 1MB Safety Cap

type WebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// WebhookHandler provides a memory-bounded, constant-time authenticated endpoint.
func WebhookHandler(expectedToken string, reloadCallback func(*PastaayConfig)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", "")
			return
		}

		defer r.Body.Close()

		// Timing-attack resistant token validation.
		reqToken := r.Header.Get("X-Pastaay-Token")
		if expectedToken != "" && subtle.ConstantTimeCompare([]byte(reqToken), []byte(expectedToken)) != 1 {
			log.Printf("[Pastaay-Webhook] Blocked unauthorized request from %s", r.RemoteAddr)
			writeJSONError(w, http.StatusUnauthorized, "Invalid token", "")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookPayloadBytes))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Payload read error or too large", "")
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
