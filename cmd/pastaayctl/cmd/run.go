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
	runCmd.Flags().IntVar(&failThreshold, "threshold", 3, "Failure threshold before auto-rollback")
}

func runExperiment(cmd *cobra.Command, args []string) {
	// Signal trapping
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("%s[#] STARTING CONTROLLED EXPERIMENT...%s\n", cBold+cCyan, cReset)

	healthURL := expHealthURL
	if healthURL == "" {
		if strings.Contains(targetURL, ":2112/metrics") {
			healthURL = strings.Replace(targetURL, ":2112/metrics", ":8080/api/v1/ping", 1)
		} else {
			fmt.Printf("\n%s[!] FATAL: Custom target detected. You MUST provide a valid health check endpoint in production!%s\n", cRed, cReset)
			fmt.Printf("%s[!] Example: pastaayctl run %s --target %s --health-url http://api.com/health (REQUIRES EXPLICIT HEALTH URL LOGIC)%s\n", cGray, args[0], targetURL, cReset)
			os.Exit(1)
		}
	}

	// Pre-Flight Check
	if ok, _ := probe(healthURL); !ok {
		fmt.Printf("%s[!] ABORTED: System is already unhealthy or health URL is unreachable.%s\n", cRed, cReset)
		return
	}

	// Rollback
	defer func() {
		fmt.Printf("\n%s[*] Finalizing: Triggering Atomic Rollback...%s\n", cCyan, cReset)
		runRollback(nil, nil)
	}()

	// Inject Chaos
	content, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("\n%s[!] ABORTED: Failed to read policy file: %v%s\n", cRed, err, cReset)
		return
	}

	dispatch(content)

	// Probing Loop
	ticker := time.NewTicker(expInterval)
	defer ticker.Stop()

	expTimer := time.NewTimer(expDuration)
	defer expTimer.Stop()

	failures, start := 0, time.Now()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n" + cYellow + "[!] Interrupted by operator." + cReset)
			return
		case <-expTimer.C:
			fmt.Printf("\n%s[+] SUCCESS: System survived the experiment.%s\n", cGreen, cReset)
			return
		case <-ticker.C:
			ok, latency := probe(healthURL)
			if !ok || latency > maxLatency {
				failures++
				fmt.Printf("\n%s[!] Degraded Performance (%v) [%d/%d]%s", cRed, latency.Round(time.Millisecond), failures, failThreshold, cReset)
				if failures >= failThreshold {
					fmt.Printf("\n%s[!!!] SLA BREACHED: Emergency shutdown triggered.%s\n", cBold+cRed, cReset)

					runRollback(nil, nil)
					os.Exit(1)
				}
			} else {
				failures = 0
				fmt.Printf("  %s[%v]%s Health: OK (%v)\r", cGray, time.Since(start).Round(time.Second), cReset, latency.Round(time.Millisecond))
			}
		}
	}
}

func probe(url string) (bool, time.Duration) {
	client := http.Client{Timeout: 2 * time.Second}
	start := time.Now()
	resp, err := client.Get(url)

	if err != nil {
		return false, time.Since(start)
	}

	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 500 {
		return false, time.Since(start)
	}

	return true, time.Since(start)
}
