package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func buildJSONRequest(method, url string, payload interface{}) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("payload marshal: %w", err)
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func executeRequest(req *http.Request, provider string, parser func([]byte) (string, error)) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s transport: %w", provider, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s body read: %w", provider, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s API error %d: %s", provider, resp.StatusCode, truncate(string(body), 512))
	}
	text, parseErr := parser(body)
	if parseErr != nil {
		log.Printf("[Pastaay-Oracle] %s parse failure: %v", provider, parseErr)
		return "", parseErr
	}
	return text, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

func ExtractYAML(response string) string {
	start := strings.Index(response, "```yaml")
	if start == -1 {
		return ""
	}
	start += 7
	end := strings.Index(response[start:], "```")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(response[start : start+end])
}
