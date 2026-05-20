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

var (
	expDuration   time.Duration
	expInterval   time.Duration
	expHealthURL  string
	maxLatency    time.Duration
	failThreshold int
)

// Reusable probe client — avoids per-call socket churn.
var probeClient = &http.Client{Timeout: 2 * time.Second}

var runCmd = &cobra.Command{
	Use:   "run [file.yaml]",
	Short: "Automated Experiment: Resilient injection with SLA guarding",
	Args:  cobra.ExactArgs(1),
	Run:   runExperiment,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().DurationVarP(&expDuration, "duration", "d", 30*time.Second, "Total experiment duration")
	runCmd.Flags().DurationVarP(&expInterval, "interval", "i", 2*time.Second, "Health probe frequency")
	runCmd.Flags().StringVar(&expHealthURL, "health-url", "", "Custom health check URL")
	runCmd.Flags().DurationVar(&maxLatency, "max-latency", 500*time.Millisecond, "SLA: Abort if latency exceeds this")
	runCmd.Flags().IntVar(&failThreshold, "threshold", 3, "Failure threshold before auto-halt")
}

func runExperiment(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("%s[#] STARTING CONTROLLED EXPERIMENT...%s\n", cBold+cCyan, cReset)

	healthURL := expHealthURL
	if healthURL == "" {
		if strings.Contains(targetURL, ":2112/metrics") {
			healthURL = strings.Replace(targetURL, ":2112/metrics", ":8080/api/v1/ping", 1)
		} else {
			fmt.Printf("\n%s[!] FATAL: Custom target detected. You MUST provide a valid health check endpoint in production!%s\n", cRed, cReset)
			os.Exit(1)
		}
	}

	if ok, _, _ := probe(ctx, healthURL); !ok {
		fmt.Printf("%s[!] ABORTED: System is already unhealthy or health URL is unreachable.%s\n", cRed, cReset)
		return
	}

	defer func() {
		fmt.Printf("\n%s[*] Finalizing: Triggering Atomic Halt...%s\n", cCyan, cReset)
		runHalt(nil, nil)
	}()

	content, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("\n%s[!] ABORTED: Failed to read policy file: %v%s\n", cRed, err, cReset)
		return
	}

	dispatch(content)

	ticker := time.NewTicker(expInterval)
	defer ticker.Stop()
	expTimer := time.NewTimer(expDuration)
	defer expTimer.Stop()

	var totalProbes, errorCount, latencyCount float64
	alpha, beta := 1.5, 0.5
	failures, start := 0, time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n" + cYellow + "[!] Interrupted by operator." + cReset)
			printResilienceScore(totalProbes, errorCount, latencyCount, alpha, beta)
			return
		case <-expTimer.C:
			fmt.Printf("\n\n%s[+] SUCCESS: System survived the experiment.%s\n", cGreen, cReset)
			printResilienceScore(totalProbes, errorCount, latencyCount, alpha, beta)
			return
		case <-ticker.C:
			totalProbes++
			ok, latency, isError := probe(ctx, healthURL)

			if isError {
				errorCount++
			} else if latency > maxLatency {
				latencyCount++
			}

			if !ok || latency > maxLatency {
				failures++
				fmt.Printf("\n%s[!] Degraded Performance (%v) [%d/%d]%s", cRed, latency.Round(time.Millisecond), failures, failThreshold, cReset)
				if failures >= failThreshold {
					fmt.Printf("\n%s[!!!] SLA BREACHED: Emergency shutdown triggered.%s\n", cBold+cRed, cReset)
					printResilienceScore(totalProbes, errorCount, latencyCount, alpha, beta)
					runHalt(nil, nil)
					os.Exit(1)
				}
			} else {
				failures = 0
				fmt.Printf("  %s[%v]%s Health: OK (%v)\r", cGray, time.Since(start).Round(time.Second), cReset, latency.Round(time.Millisecond))
			}
		}
	}
}

func printResilienceScore(total, errs, lats, alpha, beta float64) {
	if total == 0 {
		return
	}

	penalty := ((alpha * errs) + (beta * lats)) / total
	score := (1.0 - penalty) * 100

	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	grade, color := "A+", cGreen
	if score < 50 {
		grade, color = "F (Critical Failure)", cRed
	} else if score < 75 {
		grade, color = "C (Degraded)", cYellow
	} else if score < 90 {
		grade, color = "B (Stable)", cCyan
	}

	fmt.Printf("\n%s═══ POST-MORTEM RESILIENCE ANALYSIS ═══%s\n", cBold+cPurple, cReset)
	fmt.Printf("Total Probes    : %.0f\n", total)
	fmt.Printf("Error Count (Er): %.0f (Penalty x%.1f)\n", errs, alpha)
	fmt.Printf("Latency (Lr)    : %.0f (Penalty x%.1f)\n", lats, beta)
	fmt.Printf("Final Score     : %s%.2f / 100 [%s]%s\n\n", color, score, grade, cReset)
}

func probe(ctx context.Context, url string) (bool, time.Duration, bool) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, time.Since(start), true
	}

	resp, err := probeClient.Do(req)
	if err != nil {
		return false, time.Since(start), true
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 500 {
		return false, time.Since(start), true
	}

	return true, time.Since(start), false
}
