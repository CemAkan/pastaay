package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	autoStep      float64
	autoLimit     float64
	autoWait      time.Duration
	autoHealthURL string
	autoProtocol  string
)

var autopilotCmd = &cobra.Command{
	Use:   "autopilot",
	Short: "Adaptive Resilience: Auto-scale stress until SLA breach",
	Long:  cCyan + "Autopilot mode intelligently searches for your system's breaking point." + cReset,
	Run:   runAutopilot,
}

func init() {
	rootCmd.AddCommand(autopilotCmd)
	autopilotCmd.Flags().Float64Var(&autoStep, "step", 0.05, "Probability increment per cycle")
	autopilotCmd.Flags().Float64Var(&autoLimit, "limit", 0.6, "Maximum failure probability ceiling")
	autopilotCmd.Flags().DurationVar(&autoWait, "settle-time", 8*time.Second, "Time to wait for system stability")
	autopilotCmd.Flags().StringVar(&autoHealthURL, "health-url", "", "Custom health check URL")
	autopilotCmd.Flags().StringVar(&autoProtocol, "protocol", "http", "Target protocol (http, grpc, sql, redis, mongo, kafka, rabbitmq)")
}

func runAutopilot(cmd *cobra.Command, args []string) {
	fmt.Printf("%s ENGAGING ADAPTIVE AUTOPILOT...%s\n", cBold+cCyan, cReset)

	healthURL, _ := cmd.Flags().GetString("health-url")
	if healthURL == "" {
		if strings.Contains(targetURL, ":2112/metrics") {
			healthURL = strings.Replace(targetURL, ":2112/metrics", ":8080/api/v1/ping", 1)
		} else {
			fmt.Printf("\n%s[!] FATAL SRE GUARD: Custom target detected. You MUST provide a valid health check endpoint using --health-url !%s\n", cRed, cReset)
			os.Exit(1)
		}
	}

	currentChance := 0.05
	startTime := time.Now()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer func() {
		fmt.Printf("\n%s[*] Autopilot shutting down. Cleaning active policies...%s\n", cCyan, cReset)
		runHalt(nil, nil)
	}()

	var totalProbes, errorCount, latencyCount float64
	alpha, beta := 1.5, 0.5

	for currentChance <= autoLimit {
		fmt.Printf("\n%s%s[ CYCLE ]%s Scaling error chance to %s%.0f%%%s\n", cBold, cGray, cReset, cYellow, currentChance*100, cReset)

		policy := map[string]interface{}{
			"name":         "autopilot-ramp",
			"type":         autoProtocol,
			"target":       "all",
			"error_chance": currentChance,
			"error_code":   503,
		}
		payload := map[string]interface{}{
			"version": 1, "policies": []interface{}{policy},
		}
		jsonBytes, _ := json.Marshal(payload)

		dispatch(jsonBytes)

		fmt.Printf("  %s[*] Stabilizing network (%v)...%s ", cGray, autoWait, cReset)

		waitTimer := time.NewTimer(autoWait)
		select {
		case <-ctx.Done():
			waitTimer.Stop()
			fmt.Printf("\n%s[!] Operator Aborted Autopilot.%s\n", cRed, cReset)
			printResilienceScore(totalProbes, errorCount, latencyCount, alpha, beta)
			return
		case <-waitTimer.C:
		}

		ok, latency, isError := probe(ctx, healthURL)
		totalProbes++

		if isError {
			errorCount++
		} else if latency > 400*time.Millisecond {
			latencyCount++
		}

		if !ok || latency > 400*time.Millisecond {
			fmt.Printf("%s[SLA BREACHED]%s\n", cRed, cReset)
			fmt.Printf("\n%s═══ FATAL LIMIT DETECTED ═══%s\n", cBold+cRed, cReset)
			fmt.Printf("%s\"Only those who could risk going too far can possibly know how far they can go.\" - W.B.%s\n\n", cPurple, cReset)
			fmt.Printf("Critical Load: %.0f%%\nRecovery: Initiated\n", currentChance*100)

			recordStrike("autopilot", "ramp-test", "FAILED")
			printResilienceScore(totalProbes, errorCount, latencyCount, alpha, beta)

			runHalt(nil, nil)
			os.Exit(1)
		}

		fmt.Printf("%s[STABLE]%s Latency: %v\n", cGreen, cReset, latency.Round(time.Millisecond))
		currentChance += autoStep
	}

	fmt.Printf("\n%s[+] MISSION SUCCESS:%s System is resilient up to %.0f%%\n", cBold+cGreen, cReset, autoLimit*100)
	fmt.Printf("Total Duration: %v\n", time.Since(startTime).Round(time.Second))

	recordStrike("autopilot", "ramp-test", "SUCCESS")
	printResilienceScore(totalProbes, errorCount, latencyCount, alpha, beta)
}
