package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"
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
		"db-outage": "version: 1\npolicies:\n  - name: db-kill\n    type: sql\n    target: all\n    drop_connection: true",
		"meltdown":  "version: 1\npolicies:\n  - name: resource-leak\n    type: resource\n    target: host\n    ram_chunk_mb: 256\n    ram_interval: 1s",
	}

	if val, ok := templates[strings.ToLower(args[0])]; ok {
		fmt.Println(val)
	} else {
		fmt.Printf("Unknown scenario. Available: db-outage, meltdown\n")
	}
}

func runExport(cmd *cobra.Command, args []string) {

	url := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/export"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)

	if err != nil {
		fmt.Printf("\033[31m[!] Export Failed: Target Unreachable (%v)\033[0m\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\033[31m[!] Export Failed: Server returned HTTP %d\033[0m\n", resp.StatusCode)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("%s\n", string(body))
}
