package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/spf13/cobra"
)

// guardCmd acts as the parent for all safety and simulation tools
var guardCmd = &cobra.Command{
	Use:   "guard",
	Short: "Safety & Strategy: Lint, plan, and validate chaos policies",
}

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

//  Logic Implementations

func runLint(cmd *cobra.Command, args []string) {
	cfg := loadAndCheck(args[0])
	violations := 0
	targets := make(map[string]string)

	for _, p := range cfg.Policies {
		key := p.Type + ":" + p.Target

		// Conflict Detection
		if original, exists := targets[key]; exists {
			fmt.Printf("%s[FAIL]%s Logical Conflict: '%s' overlaps with '%s' on %s\n", cRed, cReset, p.Name, original, key)
			violations++
		}
		targets[key] = p.Name

		// Outage vs Jitter Bounds
		if p.LatencyDuration > 10*time.Second {
			fmt.Printf("%s[FAIL]%s %s: Latency duration triggers hard timeouts.\n", cRed, cReset, p.Name)
			violations++
		}

		// OOM Safety Guard
		if p.Type == "resource" && p.RAMChunkMB > 4096 {
			fmt.Printf("%s[WARN]%s %s: Allocation chunk (>4GB) risks immediate OOM.\n", cYellow, cReset, p.Name)
			violations++
		}
	}

	if violations == 0 {
		fmt.Printf("%s[+] Linter passed: Policy follows SRE safety standards.%s\n", cGreen, cReset)
	} else {
		fmt.Printf("\n%s[!] SRE Check Failed: Found %d issues.%s\n", cRed, violations, cReset)
		os.Exit(1)
	}
}

func runPlan(cmd *cobra.Command, args []string) {
	cfg := loadAndCheck(args[0])
	fmt.Printf("%s[#] BLAST RADIUS FORECAST: %s%s\n", cBold+cCyan, args[0], cReset)

	totalIntensity := 0.0
	for _, p := range cfg.Policies {
		// Weighted Risk Score: (Error * 0.7) + (Latency * 0.3)
		intensity := (p.ErrorChance * 0.7) + (p.LatencyChance * 0.3)
		if p.DropConnection {
			intensity = 1.0
		}
		totalIntensity += intensity

		status, color := "LOW", cGreen
		if intensity > 0.6 {
			status, color = "CRITICAL", cRed
		} else if intensity > 0.3 {
			status, color = "MEDIUM", cYellow
		}

		fmt.Printf("[%s%-8s%s] %-10s -> %-15s | Risk: %.2f\n", color, status, cReset, p.Type, p.Target, intensity)
	}

	// Observability Throughput Prediction
	fmt.Printf("\n%s[ OBSERVABILITY FORECAST ]%s\n", cBold, cReset)
	fmt.Printf("  Expected Span Throughput: ~%d spans/sec\n", len(cfg.Policies)*15)
	if totalIntensity > 2.0 {
		fmt.Printf("  Infrastructure Load: %sSEVERE - High Risk%s\n", cRed, cReset)
	} else {
		fmt.Printf("  Infrastructure Load: %sSTABLE - Safe Bounds%s\n", cGreen, cReset)
	}
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
