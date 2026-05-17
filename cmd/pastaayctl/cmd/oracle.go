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
	aiProvider string
	aiKey      string
)

const systemPrompt = `You are Pastaay Oracle, an elite Site Reliability Engineering (SRE) and Chaos Engineering AI. 
Your goal is to analyze the user's distributed system topology, current metric throughput, and active chaos policies. 
You must identify weak points, suggest optimal "blast radius" configurations, and generate highly targeted Pastaay YAML configurations.
Never suggest random guesses; always base your decisions on the provided telemetry data. Be concise, technical, and output YAML in markdown blocks.`

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

	// Combine user prompt with live system reality
	finalPrompt := fmt.Sprintf("User Request: %s\n\n--- LIVE SYSTEM CONTEXT ---\n%s", userPrompt, sysContext)

	response, err := callOpenAIFormat(apiKey, systemPrompt, finalPrompt)
	if err != nil {
		fmt.Printf("\n%s[!] Oracle Link Severed: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s═══ ORACLE ANALYSIS ═══%s\n", cBold+cPurple, cReset)
	fmt.Println(response)
}

// gatherSystemContext pulls live YAML and topology data from the Pastaay Engine
func gatherSystemContext(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("ACTIVE POLICIES:\n")

	exportURL := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/export"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, exportURL, nil)
	if resp, err := telemetryClient.Do(req); err == nil {
		body, _ := io.ReadAll(resp.Body)
		sb.WriteString(string(body))
		resp.Body.Close()
	}

	sb.WriteString("\nDISCOVERED TOPOLOGY:\n")
	reqMetrics, _ := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if resp, err := telemetryClient.Do(reqMetrics); err == nil {
		scanner := bufio.NewScanner(resp.Body)
		targets := make(map[string]bool)
		for scanner.Scan() {
			if m := targetLabelRegex.FindStringSubmatch(scanner.Text()); len(m) == 2 {
				targets[m[1]] = true
			}
		}
		resp.Body.Close()
		for t := range targets {
			sb.WriteString("- " + t + "\n")
		}
	}
	return sb.String()
}

// callOpenAIFormat uses the standard OpenAI REST schema (compatible with many providers)
func callOpenAIFormat(apiKey, sysPrompt, userPrompt string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	payload := map[string]interface{}{
		"model": "gpt-4-turbo-preview", // Or any equivalent model
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3, // Low temperature for SRE predictability
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
