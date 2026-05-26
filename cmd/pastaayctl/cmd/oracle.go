package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CemAkan/pastaay/pkg/oracle"
	"github.com/spf13/cobra"
)

var (
	aiProvider      string
	aiKey           string
	oracleHealthURL string
	aiModel         string
	aiIntensity     string
)

var oracleCmd = &cobra.Command{
	Use:   "oracle [prompt]",
	Short: "AI SRE Copilot: Analyze telemetry and generate optimal blast radius configs",
	Long:  "\x1b[36m" + splash + "\x1b[0m" + "\n\n\x1b[90m" + "Pastaay Oracle leverages LLMs to autonomously analyze your fleet and design chaos experiments." + "\x1b[0m",
	Args:  cobra.MinimumNArgs(1),
	Run:   runOracle,
}

func init() {
	rootCmd.AddCommand(oracleCmd)
	oracleCmd.Flags().StringVar(&aiProvider, "provider", "openai", "AI Provider (openai, deepseek, gemini, anthropic)")
	oracleCmd.Flags().StringVar(&aiKey, "api-key", "", "API Key for the provider (falls back to PASTAAY_AI_KEY env var)")
	oracleCmd.Flags().StringVar(&oracleHealthURL, "health-url", "", "Custom health check URL for baseline latency calculation")
	oracleCmd.Flags().StringVarP(&aiModel, "model", "m", "", "Specific AI model to use (falls back to provider default)")
	oracleCmd.Flags().StringVar(&aiIntensity, "intensity", "high", "Chaos intensity level: low, medium, high, nuke")
}

func runOracle(cmd *cobra.Command, args []string) {
	const (
		clrReset  = "\033[0m"
		clrRed    = "\033[31m"
		clrYellow = "\033[33m"
		clrPurple = "\033[35m"
		clrGray   = "\033[90m"
	)

	fmt.Printf("%s[#] WAKING PASTAAY ORACLE...%s\n", "\x1b[1m"+clrPurple, clrReset)
	fmt.Printf("  %s[*] \"We're pushing the boundaries of all that is real and possible.\"%s\n", clrGray, clrReset)

	apiKey := aiKey
	if apiKey == "" {
		apiKey = os.Getenv("PASTAAY_AI_KEY")
	}
	if apiKey == "" {
		fmt.Printf("\n%s[!] Authentication Failed: No API key provided.%s\n", clrRed, clrReset)
		os.Exit(1)
	}

	userPrompt := strings.Join(args, " ")

	// Bounded context
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Printf("  %s[*] Scanning fleet topology and active kinetic state...%s\n", clrGray, clrReset)
	sysContext := gatherSystemContext(ctx)

	fmt.Printf("  %s[*] Establishing neural link with AI backend...%s\n", clrGray, clrReset)

	provider := strings.ToLower(aiProvider)
	modelToUse := aiModel
	if modelToUse == "" {
		switch provider {
		case "openai":
			modelToUse = "gpt-4o-mini"
		case "deepseek":
			modelToUse = "deepseek-reasoner"
		case "gemini":
			modelToUse = "gemini-2.5-flash"
		case "anthropic":
			modelToUse = "claude-sonnet-4-6"
		default:
			modelToUse = "gpt-4o-mini"
		}
	}

	response, err := oracle.AskOracle(provider, apiKey, modelToUse, aiIntensity, userPrompt, sysContext)
	if err != nil {
		fmt.Printf("\n%s[!] Oracle Link Severed: %v%s\n", clrRed, err, clrReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s═══ ORACLE ANALYSIS ═══%s\n", "\x1b[1m"+clrPurple, clrReset)
	fmt.Println(response)

	yamlBlock := oracle.ExtractYAML(response)
	if yamlBlock == "" {
		return
	}

	fmt.Printf("\n%s[?] Oracle has generated a Chaos Policy. Inject it into the fleet now? (y/N): %s", clrYellow, clrReset)
	reader := bufio.NewReader(os.Stdin)
	choice, readErr := reader.ReadString('\n')
	if readErr != nil && readErr != io.EOF {
		fmt.Printf("\n%s[!] Input read failed: %v%s\n", clrRed, readErr, clrReset)
		return
	}
	choice = strings.TrimSpace(strings.ToLower(choice))

	if choice == "y" || choice == "yes" {
		fmt.Printf("  %s[*] Discarding safety protocols. Injecting Oracle payload...%s\n", clrGray, clrReset)
		dispatch([]byte(yamlBlock))
	} else {
		fmt.Printf("  %s[*] Injection aborted by operator.%s\n", clrGray, clrReset)
	}
}

// gatherSystemContext fetches the live active-policy YAML from the engine /chaos/export endpoint.
func gatherSystemContext(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("ACTIVE POLICIES (YAML):\n")
	exportURL := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/export"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, exportURL, nil)
	if err != nil {
		sb.WriteString(fmt.Sprintf("(context unavailable: request build failed: %v)\n", err))
		return sb.String()
	}

	if authToken != "" {
		req.Header.Set("X-Pastaay-Token", authToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		sb.WriteString(fmt.Sprintf("(context unavailable: engine unreachable at %s: %v)\n", exportURL, err))
		return sb.String()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		sb.WriteString(fmt.Sprintf("(context unavailable: engine returned HTTP %d)\n", resp.StatusCode))
		return sb.String()
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		sb.WriteString(fmt.Sprintf("(context unavailable: response read failed: %v)\n", readErr))
		return sb.String()
	}
	sb.Write(body)
	return sb.String()
}
