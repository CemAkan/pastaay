package cmd

import (
	"fmt"
	"os"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/guard"
	"github.com/spf13/cobra"
)

// Sub-Command: Lint
var lintCmd = &cobra.Command{
	Use:   "lint [file.yaml]",
	Short: "Policy Linter: Detect logical conflicts and SRE violations",
	Args:  cobra.ExactArgs(1),
	Run:   runLint,
}

// Sub-Command: Plan
var planCmd = &cobra.Command{
	Use:   "plan [file.yaml]",
	Short: "Impact Planner: Forecast blast radius and observability load",
	Args:  cobra.ExactArgs(1),
	Run:   runPlan,
}

// Sub-Command: Validate
var validateCmd = &cobra.Command{
	Use:   "validate [file.yaml]",
	Short: "Schema Guard: Ensure structural integrity of the YAML",
	Args:  cobra.ExactArgs(1),
	Run:   runValidate,
}

func init() {
	rootCmd.AddCommand(lintCmd, planCmd, validateCmd)
}

// Logic Implementations
func runLint(cmd *cobra.Command, args []string) {
	cfg := loadAndCheck(args[0])
	res := guard.Analyze(cfg)

	if len(res.Issues) == 0 {
		fmt.Printf("%s[+] Linter passed: Policy follows SRE safety standards.%s\n", cGreen, cReset)
	} else {
		fmt.Printf("\n%s[!] SRE Check Failed: Found %d issues.%s\n", cRed, len(res.Issues), cReset)
		for _, issue := range res.Issues {
			fmt.Printf("  - %s\n", issue)
		}
		os.Exit(1)
	}
}

func runPlan(cmd *cobra.Command, args []string) {
	cfg := loadAndCheck(args[0])
	fmt.Printf("%s[#] BLAST RADIUS FORECAST: %s%s\n", cBold+cCyan, args[0], cReset)

	res := guard.Analyze(cfg)

	// Print Individual
	for _, p := range cfg.Policies {
		intensity := (p.ErrorChance * 0.7) + (p.LatencyChance * 0.3)
		if p.DropConnection {
			intensity = 1.0
		}
		fmt.Printf("[%s] %-10s -> %-15s | Weight: %.2f\n", cCyan, p.Type, p.Target, intensity)
	}

	fmt.Printf("\n%s[ OBSERVABILITY FORECAST ]%s\n", cBold, cReset)
	color := cGreen
	if res.Status == "CRITICAL" {
		color = cRed
	} else if res.Status == "HIGH" {
		color = cYellow
	}

	fmt.Printf("  Total Impact Score: %.2f\n", res.TotalRisk)
	fmt.Printf("  Infrastructure Load: %s%s%s\n", color, res.Status, cReset)
}

func runValidate(cmd *cobra.Command, args []string) {
	cfg := loadAndCheck(args[0])
	if err := cfg.Validate(); err != nil {
		fmt.Printf("%s[!] Structural Integrity Guard Triggered:%s\n%s%v%s\n", cRed, cReset, cYellow, err, cReset)
		os.Exit(1)
	}
	fmt.Printf("%s[+] Schema is structurally valid.%s\n", cGreen, cReset)
}

// Shared Helper
func loadAndCheck(path string) *config.PastaayConfig {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		fmt.Printf("%s[!] Failed to load policy: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}
	return cfg
}
