package cmd

import (
	"fmt"
	"os"

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
	Long:  cCyan + "Pastaay Oracle leverages LLMs (Gemini, GPT, Claude) to autonomously analyze your fleet and design chaos experiments." + cReset,
	Args:  cobra.MinimumNArgs(0),
	Run:   runOracle,
}

func init() {
	rootCmd.AddCommand(oracleCmd)
	oracleCmd.Flags().StringVar(&aiProvider, "provider", "gemini", "AI Provider to use (gemini, openai, anthropic)")
	oracleCmd.Flags().StringVar(&aiKey, "api-key", "", "API Key for the provider (falls back to PASTAAY_AI_KEY env var)")
}

func runOracle(cmd *cobra.Command, args []string) {
	fmt.Printf("%s[#] WAKING PASTAAY ORACLE (%s)...%s\n", cBold+cPurple, aiProvider, cReset)

	// Resolve API Key
	apiKey := aiKey
	if apiKey == "" {
		apiKey = os.Getenv("PASTAAY_AI_KEY")
	}

	if apiKey == "" {
		fmt.Printf("\n%s[!] Authentication Failed: No API key provided.%s\n", cRed, cReset)
		fmt.Printf("Use --api-key flag or export PASTAAY_AI_KEY environment variable.\n")
		os.Exit(1)
	}

	// Fetch Live Telemetry (Topology & Active Policies)
	fmt.Printf("  %s[*] Scanning fleet topology and active kinetic state...%s\n", cGray, cReset)
	// TODO: Fetch telemetry logic (Next Step)

	// Connect to AI Provider
	fmt.Printf("  %s[*] Establishing neural link with %s...%s\n", cGray, aiProvider, cReset)
	// TODO: Provider integration logic (Next Step)

	fmt.Printf("\n%s[+] Oracle framework initialized. Awaiting integration modules.%s\n", cGreen, cReset)
}
