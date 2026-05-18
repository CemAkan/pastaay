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

func AskOracle(provider, key, model, intensity, userPrompt, sysContext string) (string, error) {
	intensityGuide := ""
	switch strings.ToLower(intensity) {
	case "low":
		intensityGuide = "INTENSITY LEVEL LOW: Use low probabilities (0.1-0.2) and very mild latency (100ms-300ms). Do NOT drop connections."
	case "medium":
		intensityGuide = "INTENSITY LEVEL MEDIUM: Use moderate probabilities (0.3-0.6) and noticeable latency (500ms-2s)."
	case "high":
		intensityGuide = "INTENSITY LEVEL HIGH: Use severe probabilities (0.7-0.9), extreme latency (3s-8s), and enable drop_connection."
	case "nuke":
		intensityGuide = "INTENSITY LEVEL NUKE: MAXIMUM DESTRUCTION. Use 1.0 probabilities, 15s+ latencies, drop ALL connections, and trigger brutal resource starvation (RAM/CPU)."
	default:
		intensityGuide = "Use moderate SRE chaos parameters."
	}

	systemPrompt := "You are Pastaay Oracle, a Senior Site Reliability Engineering (SRE) AI.\n" +
		"Analyze the provided telemetry and system configuration matrices.\n" +
		"Your ONLY output must be a highly complex, devastating, multi-layered Chaos Engineering blueprint in valid Pastaay V1 YAML wrapped in a markdown yaml block.\n\n" +
		"CRITICAL DIRECTIVES:\n" +
		"1. Output ONLY valid Pastaay V1 YAML wrapped in a markdown yaml block (using triple backticks and yaml specifier). NO conversational text.\n" +
		"2. DO NOT write single-policy basic tests. Generate a Multi-Vector Attack containing at least 3 concurrent policies.\n" +
		"3. " + intensityGuide + "\n" +
		"4. STRICT SCHEMA RULES:\n" +
		"   - NEVER use ranges for durations (e.g., '5s-15s' is ILLEGAL. Use exactly '5s' or '15s').\n" +
		"   - NEVER invent types like 'multi'. Stick EXACTLY to the provided schema below.\n" +
		"   - FATAL GUARD: For 'resource' type policies, NEVER exceed ram_chunk_mb: 512.\n" +
		"   - CLEAN YAML RULE: NEVER output `error_chance: 0` or `latency_chance: 0`. If a probability is 0, completely OMIT the field from the YAML!\n" +
		"   - RESOURCE TYPE RULE: For 'resource' policies, completely OMIT `error_chance`, `error_code`, `latency_chance`, and `drop_connection`. They are invalid for OS-level sabotage. ONLY use `latency_duration` (which controls attack length), `throttle_threshold`, `ram_chunk_mb`, and `ram_interval`.\n\n" +
		"SCHEMA FORMAT:\n" +
		"version: 1\n" +
		"warmup_duration: 10s\n" +
		"enable_default_ignored: true\n" +
		"policies:\n" +
		"  - name: <aggressive-scenario-name>\n" +
		"    type: <http|sql|grpc|redis|mongo|kafka|rabbitmq|resource>\n" +
		"    target: <specific-target-from-metrics>\n" +
		"    latency_chance: <0.1-1.0> # OMIT IF 0\n" +
		"    latency_duration: <duration>\n" +
		"    error_chance: <0.1-1.0> # OMIT IF 0\n" +
		"    error_code: <int>\n" +
		"    drop_connection: <bool>\n" +
		"    throttle_threshold: <int> # RESOURCE ONLY\n" +
		"    ram_chunk_mb: <int> # RESOURCE ONLY\n" +
		"    ram_interval: <duration> # RESOURCE ONLY"

	finalPrompt := fmt.Sprintf("User Request: %s\n\n--- LIVE TELEMETRY MATRIX ---\n%s", userPrompt, sysContext)

	switch strings.ToLower(provider) {
	case "openai":
		if model == "" {
			model = "gpt-4o-mini"
		}
		return callLLM(key, model, "https://api.openai.com/v1/chat/completions", systemPrompt, finalPrompt, "openai")
	case "deepseek":
		if model == "" {
			model = "deepseek-reasoner"
		}
		return callLLM(key, model, "https://api.deepseek.com/v1/chat/completions", systemPrompt, finalPrompt, "deepseek")
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

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

func callLLM(apiKey, model, url, sysPrompt, userPrompt, provider string) (string, error) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.5,
	}
	req, err := buildJSONRequest(http.MethodPost, url, payload)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return executeRequest(req, provider, func(b []byte) (string, error) {
		var res struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(b, &res); err != nil {
			return "", fmt.Errorf("openai response decode: %w (raw=%q)", err, truncate(string(b), 256))
		}
		if len(res.Choices) == 0 {
			return "", fmt.Errorf("openai response had no choices (raw=%q)", truncate(string(b), 256))
		}
		return res.Choices[0].Message.Content, nil
	})
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
