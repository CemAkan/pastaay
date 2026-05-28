package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var utilCmd = &cobra.Command{
	Use:   "util",
	Short: "Toolbox: Policy generation and configuration export",
}

var generateCmd = &cobra.Command{
	Use:   "generate [scenario]",
	Short: "Blueprint: Create templates for (db-outage, cache-stampede, meltdown)",
	Args:  cobra.ExactArgs(1),
	Run:   runGenerate,
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Reverse-Engineer: Download active in-memory configuration to YAML",
	Run:   runExport,
}

func init() {
	rootCmd.AddCommand(utilCmd)
	utilCmd.AddCommand(generateCmd, exportCmd)
}

func runGenerate(cmd *cobra.Command, args []string) {
	templates := map[string]string{
		"db-outage":      "version: 1\npolicies:\n  - name: db-kill\n    type: sql\n    target: all\n    drop_connection: true\n    error_chance: 1.0",
		"meltdown":       "version: 1\npolicies:\n  - name: resource-leak\n    type: resource\n    target: host\n    ram_chunk_mb: 256\n    ram_interval: 1s\n    latency_duration: 60s",
		"cache-stampede": "version: 1\npolicies:\n  - name: redis-stampede\n    type: redis\n    target: GET\n    latency_chance: 0.8\n    latency_duration: 2s\n    error_chance: 0.2\n    error_code: 500",
	}

	if val, ok := templates[strings.ToLower(args[0])]; ok {
		fmt.Println(val)
	} else {
		fmt.Printf("Unknown scenario. Available: db-outage, meltdown, cache-stampede\n")
	}
}

func runExport(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	url := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/export"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("\033[31m[!] Export Failed: Bad target URL (%v)\033[0m\n", err)
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Printf("\033[31m[!] Export Failed: Target Unreachable (%v)\033[0m\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\033[31m[!] Export Failed: Server returned HTTP %d\033[0m\n", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("\033[31m[!] Export Failed: response read (%v)\033[0m\n", err)
		return
	}
	fmt.Printf("%s\n", string(body))
}
