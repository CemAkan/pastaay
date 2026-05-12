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

		// Timing-attack resistant token validation
		reqToken := r.Header.Get("X-Pastaay-Token")
		if expectedToken != "" && subtle.ConstantTimeCompare([]byte(reqToken), []byte(expectedToken)) != 1 {
			log.Printf("[Pastaay-Webhook] Blocked unauthorized request from %s", r.RemoteAddr)
			writeJSONError(w, http.StatusUnauthorized, "Invalid token", "")
			return
		}

		// Body closure is critical for preventing socket leaks
		defer r.Body.Close()

		// Memory Guard: Explicitly limiting the reader before any allocation
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
			log.Printf("[Pastaay-Webhook] Rejected invalid config from %s", r.RemoteAddr)
			writeJSONError(w, http.StatusUnprocessableEntity, "Policy validation failed", err.Error())
			return
		}

		reloadCallback(&newCfg)
		log.Printf("[Pastaay-Webhook] Engine memory updated successfully")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(WebhookResponse{Status: "success", Message: "Applied"})
	}
}

func writeJSONError(w http.ResponseWriter, code int, msg, details string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(WebhookResponse{Status: "error", Message: msg, Details: details})
}
