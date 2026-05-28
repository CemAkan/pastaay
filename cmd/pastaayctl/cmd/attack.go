package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

var (
	strikeType    string
	strikeTarget  string
	strikeLatency time.Duration
	strikeLProb   float64
	strikeErrCode int
	strikeEProb   float64
	strikeDrop    bool
	strikeTTL     time.Duration
	broadcastMode bool
)

var strikeCmd = &cobra.Command{
	Use:   "strike",
	Short: "Imperative Strike: Inject chaos via flags",
	Run:   runStrike,
}

var injectCmd = &cobra.Command{
	Use:   "inject [file.yaml]",
	Short: "Payload Injection: Apply a YAML policy file",
	Args:  cobra.ExactArgs(1),
	Run:   runInject,
}

var snipeCmd = &cobra.Command{
	Use:   "snipe",
	Short: "Interactive Sniper: Guided chaos injection wizard",
	Run:   runSnipe,
}

var haltCmd = &cobra.Command{
	Use:   "halt",
	Short: "Emergency Kill Switch: Revoke all active chaos policies instantly",
	Run:   runHalt,
}

var broadcastCmd = &cobra.Command{
	Use:   "broadcast [file.yaml]",
	Short: "Fleet Broadcast: Publish policy to all nodes via Redis PubSub",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		broadcastMode = true
		runInject(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(strikeCmd, injectCmd, snipeCmd, haltCmd, broadcastCmd)

	strikeCmd.Flags().StringVarP(&strikeType, "type", "p", "http", "Protocol type")
	strikeCmd.Flags().StringVarP(&strikeTarget, "target", "t", "all", "Target endpoint")
	strikeCmd.Flags().DurationVarP(&strikeLatency, "latency", "l", 0, "Latency duration")
	strikeCmd.Flags().Float64Var(&strikeLProb, "latency-chance", 1.0, "Latency probability")
	strikeCmd.Flags().IntVarP(&strikeErrCode, "error-code", "e", 0, "Error code")
	strikeCmd.Flags().Float64Var(&strikeEProb, "error-chance", 1.0, "Error probability")
	strikeCmd.Flags().BoolVarP(&strikeDrop, "drop", "d", false, "Drop connection")
	strikeCmd.Flags().BoolVarP(&broadcastMode, "broadcast", "b", false, "Broadcast to entire fleet via Redis")
	strikeCmd.Flags().DurationVar(&strikeTTL, "ttl", 0, "Auto-halt duration (Dead man's switch)")
}

func runStrike(cmd *cobra.Command, args []string) {
	policy := map[string]interface{}{
		"name":   fmt.Sprintf("strike-%d", time.Now().Unix()),
		"type":   strikeType,
		"target": strikeTarget,
	}

	if strikeLatency > 0 {
		policy["latency_duration"] = strikeLatency.String()
		policy["latency_chance"] = strikeLProb
	}
	if strikeErrCode > 0 {
		policy["error_code"] = strikeErrCode
		policy["error_chance"] = strikeEProb
	}
	if strikeDrop {
		policy["drop_connection"] = true
		policy["error_chance"] = strikeEProb
	}

	payload := map[string]interface{}{
		"version":  1,
		"policies": []interface{}{policy},
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("%s[!] Strike payload marshal failed: %v%s\n", cRed, err, cReset)
		return
	}

	dispatch(jsonBytes)
}

func runInject(cmd *cobra.Command, args []string) {
	content, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("%s[!] Failed to read file: %v%s\n", cRed, err, cReset)
		return
	}
	dispatch(content)
}

func runSnipe(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s[#] PASTAAY SNIPER MODE%s\n", cBold+cRed, cReset)

	fmt.Print("Target (e.g. all, /api/v1/ping): ")
	t, _ := reader.ReadString('\n')
	strikeTarget = strings.TrimSpace(t)

	fmt.Print("Effect (1: Latency, 2: Error): ")
	choice, _ := reader.ReadString('\n')
	if strings.TrimSpace(choice) == "1" {
		strikeLatency = 1 * time.Second
	} else {
		strikeErrCode = 503
	}

	runStrike(cmd, args)
}

func runHalt(cmd *cobra.Command, args []string) {
	fmt.Printf("%s[!] EMERGENCY OVERRIDE INITIATED...%s\n", cYellow, cReset)
	fmt.Printf("%s[*] \"Some things are meant to be left alone...\"%s\n", cGray, cReset)
	strikeTTL = 0
	reset := []byte(`{"version": 1, "policies": []}`)
	dispatch(reset)
}

func dispatch(payload []byte) {
	if broadcastMode {
		executeRedisBroadcast(payload)
	} else {
		executeHTTPInjection(payload)
	}

	if strikeTTL > 0 {
		fmt.Printf("\n%s[*] Dead Man's Switch active. Halting in %v (Press Ctrl+C to abort early)...%s\n", cGray, strikeTTL, cReset)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		timer := time.NewTimer(strikeTTL)
		defer timer.Stop()

		select {
		case <-timer.C:
			fmt.Printf("\n%s[*] TTL Expired. Executing Auto-Halt...%s\n", cYellow, cReset)
		case <-ctx.Done():
			fmt.Printf("\n%s[!] Operator Abort detected. Executing Emergency Halt...%s\n", cBold+cRed, cReset)
		}

		runHalt(nil, nil)
	}
}

func executeHTTPInjection(payload []byte) {
	url := strings.TrimSuffix(targetURL, "/metrics") + "/chaos/webhook"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		fmt.Printf("%s[!] Failed to build request (Invalid URL?): %v%s\n", cRed, err, cReset)
		return
	}

	if bytes.HasPrefix(bytes.TrimSpace(payload), []byte("{")) {
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Content-Type", "application/yaml")
	}

	if authToken != "" {
		req.Header.Set("X-Pastaay-Token", authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("%s[!] Request Failed: %v%s\n", cRed, err, cReset)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("%s[+] PAYLOAD DELIVERED SUCCESSFULLY%s\n", cGreen, cReset)
	} else {
		msg, _ := io.ReadAll(resp.Body)
		fmt.Printf("%s[!] REJECTED (%d): %s%s\n", cRed, resp.StatusCode, string(msg), cReset)
	}
}

func executeRedisBroadcast(payload []byte) {
	addr := os.Getenv("PASTAAY_REDIS_ADDR")
	if addr == "" {
		fmt.Printf("%s[!] PASTAAY_REDIS_ADDR is not set — refusing to publish chaos policies to an implicit address.%s\n", cRed, cReset)
		fmt.Printf("    %s(set PASTAAY_REDIS_ADDR explicitly; for local dev export PASTAAY_REDIS_ADDR=localhost:6379)%s\n", cGray, cReset)
		return
	}

	opts := &redis.Options{Addr: addr}
	if pw := os.Getenv("PASTAAY_REDIS_PASSWORD"); pw != "" {
		opts.Password = pw
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Publish(ctx, "pastaay:chaos:policies", string(payload)).Err(); err != nil {
		fmt.Printf("%s[!] Redis Broadcast Failed: %v%s\n", cRed, err, cReset)
		return
	}
	fmt.Printf("%s[+] FLEET-WIDE BROADCAST COMPLETED%s\n", cGreen, cReset)
}
