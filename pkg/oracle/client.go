package oracle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxLLMResponseBytes = 4 << 20
	defaultLLMTimeout   = 30 * time.Second
	userAgent           = "pastaay-oracle/2.0"
)

var sharedLLMClient = &http.Client{Timeout: defaultLLMTimeout}

func AskOracle(provider, key, model, intensity, userPrompt, sysContext string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultLLMTimeout)
	defer cancel()
	return AskOracleCtx(ctx, provider, key, model, intensity, userPrompt, sysContext)
}

func AskOracleCtx(ctx context.Context, provider, key, model, intensity, userPrompt, sysContext string) (string, error) {
	intensityGuide := buildIntensityGuide(intensity)
	systemPrompt := buildSystemPrompt(intensityGuide)
	finalPrompt := fmt.Sprintf("User Request: %s\n\n--- LIVE TELEMETRY MATRIX ---\n%s", userPrompt, sysContext)

	switch strings.ToLower(provider) {
	case "openai":
		if model == "" {
			model = "gpt-4o-mini"
		}
		return callLLM(ctx, key, model, "https://api.openai.com/v1/chat/completions", systemPrompt, finalPrompt, "openai")
	case "deepseek":
		if model == "" {
			model = "deepseek-reasoner"
		}
		return callLLM(ctx, key, model, "https://api.deepseek.com/v1/chat/completions", systemPrompt, finalPrompt, "deepseek")
	case "gemini":
		if model == "" {
			model = "gemini-2.5-flash"
		}
		return callGemini(ctx, key, model, systemPrompt, finalPrompt)
	case "anthropic":
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		return callAnthropic(ctx, key, model, systemPrompt, finalPrompt)
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

func buildIntensityGuide(intensity string) string {
	switch strings.ToLower(intensity) {
	case "low":
		return "INTENSITY LEVEL LOW: Use low probabilities (0.1-0.2) and very mild latency (100ms-300ms). Do NOT drop connections."
	case "medium":
		return "INTENSITY LEVEL MEDIUM: Use moderate probabilities (0.3-0.6) and noticeable latency (500ms-2s)."
	case "high":
		return "INTENSITY LEVEL HIGH: Use severe probabilities (0.7-0.9), extreme latency (3s-8s), and enable drop_connection."
	case "nuke":
		return "INTENSITY LEVEL NUKE: MAXIMUM DESTRUCTION. Use 1.0 probabilities, 15s+ latencies, drop ALL connections, and trigger brutal resource starvation (RAM/CPU)."
	default:
		return "Use moderate SRE chaos parameters."
	}
}

func buildSystemPrompt(intensityGuide string) string {
	return "You are Pastaay Oracle, a Senior Site Reliability Engineering (SRE) AI.\n" +
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
		"    latency_chance: <0.1-1.0>\n" +
		"    latency_duration: <duration>\n" +
		"    error_chance: <0.1-1.0>\n" +
		"    error_code: <int>\n" +
		"    drop_connection: <bool>\n" +
		"    throttle_threshold: <int>\n" +
		"    ram_chunk_mb: <int>\n" +
		"    ram_interval: <duration>"
}

func buildJSONRequest(ctx context.Context, method, url string, payload interface{}) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("payload marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func callLLM(ctx context.Context, apiKey, model, url, sysPrompt, userPrompt, provider string) (string, error) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.5,
	}
	req, err := buildJSONRequest(ctx, http.MethodPost, url, payload)
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
			return "", fmt.Errorf("%s response decode: %w (raw=%q)", provider, err, truncate(string(b), 256))
		}
		if len(res.Choices) == 0 {
			return "", fmt.Errorf("%s response had no choices (raw=%q)", provider, truncate(string(b), 256))
		}
		return res.Choices[0].Message.Content, nil
	})
}

func callGemini(ctx context.Context, apiKey, model, sysPrompt, userPrompt string) (string, error) {
	url := "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent"
	payload := map[string]interface{}{
		"system_instruction": map[string]interface{}{"parts": map[string]string{"text": sysPrompt}},
		"contents":           []map[string]interface{}{{"parts": []map[string]string{{"text": userPrompt}}}},
		"generationConfig":   map[string]interface{}{"temperature": 0.5},
	}
	req, err := buildJSONRequest(ctx, http.MethodPost, url, payload)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", apiKey)
	return executeRequest(req, "gemini", func(b []byte) (string, error) {
		var res struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal(b, &res); err != nil {
			return "", fmt.Errorf("gemini response decode: %w (raw=%q)", err, truncate(string(b), 256))
		}
		if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("gemini response had no candidates/parts (raw=%q)", truncate(string(b), 256))
		}
		return res.Candidates[0].Content.Parts[0].Text, nil
	})
}

func callAnthropic(ctx context.Context, apiKey, model, sysPrompt, userPrompt string) (string, error) {
	payload := map[string]interface{}{
		"model":       model,
		"max_tokens":  1500,
		"system":      sysPrompt,
		"messages":    []map[string]string{{"role": "user", "content": userPrompt}},
		"temperature": 0.5,
	}
	req, err := buildJSONRequest(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", payload)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	return executeRequest(req, "anthropic", func(b []byte) (string, error) {
		var res struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(b, &res); err != nil {
			return "", fmt.Errorf("anthropic response decode: %w (raw=%q)", err, truncate(string(b), 256))
		}
		if len(res.Content) == 0 {
			return "", fmt.Errorf("anthropic response had empty content (raw=%q)", truncate(string(b), 256))
		}
		return res.Content[0].Text, nil
	})
}

func executeRequest(req *http.Request, provider string, parser func([]byte) (string, error)) (string, error) {
	resp, err := sharedLLMClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s transport: %w", provider, err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxLLMResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("%s body read: %w", provider, err)
	}
	if int64(len(body)) > maxLLMResponseBytes {
		return "", fmt.Errorf("%s response exceeded %d byte cap", provider, maxLLMResponseBytes)
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

// truncate rounds to a UTF-8 boundary. n<=0 returns only the marker.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 0 {
		return "...(truncated)"
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "...(truncated)"
}

// ExtractYAML scans the response for ```yaml or ```yml fences
func ExtractYAML(response string) string {
	for i := 0; i+2 < len(response); i++ {
		if response[i] != '`' || response[i+1] != '`' || response[i+2] != '`' {
			continue
		}
		rest := response[i+3:]
		var fenceLen int
		switch {
		case len(rest) >= 4 && strings.EqualFold(rest[:4], "yaml"):
			fenceLen = 4
		case len(rest) >= 3 && strings.EqualFold(rest[:3], "yml"):
			fenceLen = 3
		default:
			continue
		}
		start := i + 3 + fenceLen
		if start > len(response) {
			return ""
		}
		end := strings.Index(response[start:], "```")
		if end == -1 {
			return ""
		}
		return strings.TrimSpace(response[start : start+end])
	}
	return ""
}
