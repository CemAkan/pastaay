package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	aiProvider      string
	aiKey           string
	oracleHealthURL string
)

const systemPrompt = `You are Pastaay Oracle, an elite Site Reliability Engineering (SRE) and Chaos Engineering AI. 
Your goal is to analyze the user's distributed system topology, current metric throughput, and active chaos policies. 
You must identify weak points, suggest optimal "blast radius" configurations, and generate highly targeted Pastaay YAML configurations.
Never suggest random guesses; always base your decisions on the telemetry data. Be concise. Output YAML in markdown blocks.
CRITICAL: Always remind the operator that they can instantly abort the experiment by running 'pastaayctl rollback'.`

var oracleCmd = &cobra.Command{
	Use:   "oracle [prompt]",
	Short: "AI SRE Copilot: Analyze telemetry and generate optimal blast radius configs",
	Long:  cCyan + "Pastaay Oracle leverages LLMs to autonomously analyze your fleet and design chaos experiments." + cReset,
	Args:  cobra.MinimumNArgs(1),
	Run:   runOracle,
}

func init() {
	rootCmd.AddCommand(oracleCmd)
	oracleCmd.Flags().StringVar(&aiProvider, "provider", "openai", "AI Provider format to use (openai is standard for GPT/Grok/Local)")
	oracleCmd.Flags().StringVar(&aiKey, "api-key", "", "API Key for the provider (falls back to PASTAAY_AI_KEY env var)")
	oracleCmd.Flags().StringVar(&oracleHealthURL, "health-url", "", "Custom health check URL for baseline latency calculation")
}

func runOracle(cmd *cobra.Command, args []string) {
	fmt.Printf("%s[#] WAKING PASTAAY ORACLE...%s\n", cBold+cPurple, cReset)

	apiKey := aiKey
	if apiKey == "" {
		apiKey = os.Getenv("PASTAAY_AI_KEY")
	}
	if apiKey == "" {
		fmt.Printf("\n%s[!] Authentication Failed: No API key provided.%s\n", cRed, cReset)
		fmt.Printf("Use --api-key flag or export PASTAAY_AI_KEY environment variable.\n")
		os.Exit(1)
	}

	userPrompt := strings.Join(args, " ")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("  %s[*] Scanning fleet topology and active kinetic state...%s\n", cGray, cReset)
	sysContext := gatherSystemContext(ctx)

	fmt.Printf("  %s[*] Establishing neural link with AI backend...%s\n", cGray, cReset)

	finalPrompt := fmt.Sprintf("User Request: %s\n\n--- LIVE SYSTEM CONTEXT ---\n%s", userPrompt, sysContext)

	var response string
	var err error

	switch strings.ToLower(aiProvider) {
	case "openai":
		response, err = callOpenAIFormat(apiKey, systemPrompt, finalPrompt)
	case "gemini":
		response, err = callGeminiFormat(apiKey, systemPrompt, finalPrompt)
	case "anthropic":
		response, err = callAnthropicFormat(apiKey, systemPrompt, finalPrompt)
	default:
		fmt.Printf("\n%s[!] Unknown Provider: %s. Supported: openai, gemini, anthropic%s\n", cRed, aiProvider, cReset)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("\n%s[!] Oracle Link Severed: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s═══ ORACLE ANALYSIS ═══%s\n", cBold+cPurple, cReset)
	fmt.Println(response)

	yamlBlock := extractYAML(response)
	if yamlBlock != "" {
		fmt.Printf("\n%s[?] Oracle has generated a Chaos Policy. Would you like to inject it into the fleet now? (y/N): %s", cYellow, cReset)
		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(strings.ToLower(choice))

		if choice == "y" || choice == "yes" {
			fmt.Printf("  %s[*] Discarding safety protocols. Injecting Oracle payload...%s\n", cGray, cReset)
			// Reuses the existing dispatch function from attack.go!
			dispatch([]byte(yamlBlock))
		} else {
			fmt.Printf("  %s[*] Injection aborted by operator.%s\n", cGray, cReset)
		}
	}
}

// gatherSystemContext pulls live YAML, topology, fault counts, and baseline latency
func gatherSystemContext(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("ACTIVE POLICIES (YAML):\n")

	// Fetch Active Policies
	exportURL := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/export"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, exportURL, nil)
	if resp, err := telemetryClient.Do(req); err == nil {
		body, _ := io.ReadAll(resp.Body)
		sb.WriteString(string(body))
		resp.Body.Close()
	}

	// Fetch Fault Metrics & Topology
	sb.WriteString("\nKINETIC IMPACT (Faults Injected):\n")
	reqMetrics, _ := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if resp, err := telemetryClient.Do(reqMetrics); err == nil {
		scanner := bufio.NewScanner(resp.Body)
		faults := make(map[string]int)

		for scanner.Scan() {
			line := scanner.Text()
			// Re-uses faultRegex from telemetry.go
			if m := faultRegex.FindStringSubmatch(line); len(m) == 4 {
				faultType, target, countStr := m[1], m[2], m[3]
				var count int
				fmt.Sscanf(countStr, "%d", &count)

				key := fmt.Sprintf("- Target: [%s] | Fault: %s", target, faultType)
				faults[key] += count
			}
		}
		resp.Body.Close()

		if len(faults) == 0 {
			sb.WriteString("No active faults recorded yet. System is pristine.\n")
		} else {
			for t, count := range faults {
				sb.WriteString(fmt.Sprintf("%s | Total Hits: %d\n", t, count))
			}
		}
	}

	// Establish Baseline Latency (Health Probe)
	sb.WriteString("\nCURRENT BASELINE HEALTH:\n")

	healthURL := oracleHealthURL
	if healthURL == "" {
		healthURL = strings.Replace(targetURL, ":2112/metrics", ":8080/api/v1/ping", 1)
	}

	start := time.Now()
	healthReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if resp, err := telemetryClient.Do(healthReq); err == nil {
		latency := time.Since(start)
		sb.WriteString(fmt.Sprintf("- Health Check URL: %s\n- Status Code: %d\n- Round-Trip Latency: %v\n", healthURL, resp.StatusCode, latency.Round(time.Millisecond)))
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	} else {
		sb.WriteString(fmt.Sprintf("- Health Check URL: %s\n- Status: UNREACHABLE (Critical Error)\n", healthURL))
	}

	return sb.String()
}

func callOpenAIFormat(apiKey, sysPrompt, userPrompt string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	payload := map[string]interface{}{
		"model": "gpt-4-turbo-preview",
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3,
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("empty response from AI")
}

// extractYAML sifts through the AI's markdown response to find the raw YAML block
func extractYAML(response string) string {
	start := strings.Index(response, "```yaml")
	if start == -1 {
		return ""
	}
	start += 7 // Move past "```yaml"

	// Find the closing backticks after the yaml declaration
	end := strings.Index(response[start:], "```")
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(response[start : start+end])
}

// callGeminiFormat integrates with Google's Gemini API (Google AI Studio)
func callGeminiFormat(apiKey, sysPrompt, userPrompt string) (string, error) {
	// Note: Gemini puts the key in the URL query parameters
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-pro-latest:generateContent?key=" + apiKey

	// Gemini combines system prompt into the main request for standard REST
	combinedPrompt := fmt.Sprintf("SYSTEM INSTRUCTIONS:\n%s\n\nUSER REQUEST:\n%s", sysPrompt, userPrompt)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": combinedPrompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.3,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("empty response from Gemini")
}

// callAnthropicFormat integrates with Anthropic's Claude Messages API
func callAnthropicFormat(apiKey, sysPrompt, userPrompt string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	payload := map[string]interface{}{
		"model":      "claude-3-opus-20240229",
		"max_tokens": 1024,
		"system":     sysPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3,
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Anthropic API returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", fmt.Errorf("empty response from Anthropic")
}
