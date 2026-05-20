package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	faultRegex       = regexp.MustCompile(`pastaay_injected_faults_total\{fault_type="([^"]+)",target="([^"]+)"\} ([0-9]+)`)
	sensorRegex      = regexp.MustCompile(`pastaay_remote_sensor_status\{sensor="([^"]+)"\} ([0-9\.]+)`)
	targetLabelRegex = regexp.MustCompile(`target="((?:[^"\\]|\\.)*)"`)
)

var topCmd = &cobra.Command{Use: "top", Short: "Live Dashboard: Real-time kinetic view of chaos impact", Run: runTop}
var statusCmd = &cobra.Command{Use: "status", Short: "Fleet Status: Quick health check of all remote sensors", Run: runStatus}
var discoverCmd = &cobra.Command{Use: "discover", Short: "Topology Discovery: Map injectable services and endpoints", Run: runDiscover}
var inspectCmd = &cobra.Command{Use: "inspect", Short: "Memory X-Ray: View raw in-memory chaos policies", Run: runInspect}

var telemetryClient = &http.Client{Timeout: 2 * time.Second}

func init() {
	rootCmd.AddCommand(topCmd, statusCmd, discoverCmd, inspectCmd)
}

// Logic Implementations
func runTop(cmd *cobra.Command, args []string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if outputJSON {
		resp, err := fetchMetrics(ctx)
		if err != nil {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
			return
		}
		defer resp.Body.Close()
		json.NewEncoder(os.Stdout).Encode(map[string]string{"status": "metric_stream_active", "info": "use /metrics directly for json dump"})
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	fmt.Print("\033[?25l")       // Hide cursor
	defer fmt.Print("\033[?25h") // Show cursor

	lastFaults := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := fetchMetrics(ctx)
			if err != nil {
				drawOfflineScreen(err.Error())
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
				}
				continue
			}
			renderDashboard(resp, lastFaults)
			resp.Body.Close()
		}
	}
}

func runStatus(cmd *cobra.Command, args []string) {
	resp, err := fetchMetrics(context.Background())
	if err != nil {
		if outputJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Printf("%s[!] Failed to reach fleet: %v%s\n", cRed, err, cReset)
		}
		return
	}
	defer resp.Body.Close()

	type SensorData struct {
		Name     string `json:"sensor"`
		Status   string `json:"status"`
		Endpoint string `json:"endpoint"`
	}
	var data []SensorData

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		if m := sensorRegex.FindStringSubmatch(scanner.Text()); len(m) == 3 {
			statusText := "OFFLINE"
			val, _ := strconv.ParseFloat(m[2], 64)
			if val >= 1.0 {
				statusText = "ONLINE"
			}
			data = append(data, SensorData{Name: m[1], Status: statusText, Endpoint: targetURL})
		}
	}

	if outputJSON {
		json.NewEncoder(os.Stdout).Encode(data)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, cBold+"\nSENSOR\tSTATUS\tENDPOINT"+cReset)
	for _, d := range data {
		color := cRed
		if d.Status == "ONLINE" {
			color = cGreen
		}
		fmt.Fprintf(w, "[%s]\t%s%s%s\t%s\n", cCyan+strings.ToUpper(d.Name)+cReset, color, d.Status, cReset, d.Endpoint)
	}
	w.Flush()
}

func runDiscover(cmd *cobra.Command, args []string) {
	resp, err := fetchMetrics(context.Background())
	if err != nil {
		if outputJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Printf("%s[!] Topology Discovery Failed: %v%s\n", cRed, err, cReset)
		}
		return
	}
	defer resp.Body.Close()

	var targetList []string
	targets := make(map[string]bool)

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "pastaay_injected_faults_total") {
			if m := targetLabelRegex.FindStringSubmatch(line); len(m) == 2 {
				t := m[1]
				if !targets[t] {
					targets[t] = true
					targetList = append(targetList, t)
				}
			}
		}
	}

	if outputJSON {
		json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"topology": targetList})
		return
	}

	fmt.Printf("%s[*] DISCOVERING FLEET TOPOLOGY...%s\n", cCyan, cReset)
	fmt.Printf("\n%s═══ INJECTABLE TOPOLOGY ═══%s\n", cBold+cGreen, cReset)
	for _, t := range targetList {
		fmt.Printf("  %s●%s %s\n", cCyan, cReset, t)
	}
}

func runInspect(cmd *cobra.Command, args []string) {
	url := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/export"
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	resp, err := telemetryClient.Do(req)

	if err != nil {
		if outputJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Printf("%s[!] Target Unreachable: %v%s\n", cRed, err, cReset)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if outputJSON {
			json.NewEncoder(os.Stdout).Encode(map[string]string{"error": fmt.Sprintf("Server returned %d", resp.StatusCode)})
		} else {
			fmt.Printf("%s[!] Inspect Failed: Server returned HTTP %d%s\n", cRed, resp.StatusCode, cReset)
		}
		return
	}

	body, _ := io.ReadAll(resp.Body)
	if outputJSON {
		fmt.Printf("%s\n", string(body))
		return
	}
	fmt.Printf("\n%s═══ ENGINE MEMORY X-RAY ═══%s\n%s%s%s\n", cBold+cGreen, cReset, cGray, string(body), cReset)
}

// Helpers
func fetchMetrics(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid target url: %w", err)
	}
	return telemetryClient.Do(req)
}

func drawOfflineScreen(msg string) {
	fmt.Print("\033[H\033[2J") // Clear Screen
	fmt.Printf("%s[!] FLEET CONNECTION SEVERED%s\nError: %s\n", cBold+cRed, cReset, msg)
}

func renderDashboard(resp *http.Response, lastFaults map[string]int) {
	fmt.Print("\033[H\033[2J") // Clear Screen
	fmt.Printf("%sPASTAAY%s KINETIC CONTROL PLANE %sv2.0-stable%s\n", cBold+cCyan, cReset, cGray, cReset)
	fmt.Println(strings.Repeat("─", 85))
	fmt.Printf("%-35s | %-15s | %-12s | %-10s\n", "TARGET (Endpoint/Query)", "FAULT TYPE", "TOTAL HITS", "RATE (req/s)")
	fmt.Println(strings.Repeat("─", 85))

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if m := faultRegex.FindStringSubmatch(line); len(m) == 4 {
			faultType, target, totalStr := m[1], m[2], m[3]
			total, _ := strconv.Atoi(totalStr)

			key := target + "_" + faultType
			rate := 0
			if prevTotal, exists := lastFaults[key]; exists {
				if total >= prevTotal {
					rate = total - prevTotal
				}
			}
			lastFaults[key] = total

			color := cGreen
			if rate > 0 {
				color = cRed
			} else if total > 0 {
				color = cYellow
			}

			displayTarget := target
			if len(target) > 33 {
				displayTarget = target[:30] + "..."
			}

			fmt.Printf("%s%-35s%s | %-15s | %-12d | %s+%d/s%s\n", cBold, displayTarget, cReset, faultType, total, color, rate, cReset)
		}
	}
}
