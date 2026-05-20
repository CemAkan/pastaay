package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// Data schemas
type Profile struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Token  string `json:"token"`
}

type ConfigFile struct {
	CurrentContext string             `json:"current_context"`
	Profiles       map[string]Profile `json:"profiles"`
}

type StrikeEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Profile   string    `json:"profile"`
	Target    string    `json:"target"`
	Type      string    `json:"type"`
	Result    string    `json:"result"`
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit & Compliance: Review history and generate reports",
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Execution History: Review the last 15 chaos strikes",
	Run:   runHistory,
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Post-Mortem: Generate a Markdown report of the last strike",
	Run:   runReport,
}

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Profile Manager: Switch between Prod, Staging, or Local",
	Run:   func(cmd *cobra.Command, args []string) { runListProfiles() },
}

func init() {
	rootCmd.AddCommand(auditCmd, profileCmd)
	auditCmd.AddCommand(historyCmd, reportCmd)
	profileCmd.AddCommand(
		&cobra.Command{
			Use:   "add [name] [target_url]",
			Short: "Register a new environment",
			Args:  cobra.ExactArgs(2),
			Run:   runAddProfile,
		},
		&cobra.Command{
			Use:   "use [name]",
			Short: "Set active environment context",
			Args:  cobra.ExactArgs(1),
			Run:   runUseProfile,
		},
	)
}

// Profile Logic
func runListProfiles() {
	cf, err := loadConfigFile(getCfgPath())
	if err != nil {
		fmt.Printf("%s[!] Profile registry unreadable: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}

	if outputJSON {
		if err := json.NewEncoder(os.Stdout).Encode(cf.Profiles); err != nil {
			fmt.Fprintf(os.Stderr, "[!] JSON encode failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, cBold+"  \tNAME\tTARGET URL\tSTATUS"+cReset)

	for name, p := range cf.Profiles {
		mark, color := " ", ""
		if name == cf.CurrentContext {
			mark, color = ">", cGreen
		}
		fmt.Fprintf(w, " %s\t%s\t%s\t%s%s%s\n", mark, name, p.Target, color, mark, cReset)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] output flush failed: %v\n", err)
	}
}

func runAddProfile(cmd *cobra.Command, args []string) {
	path := getCfgPath()
	cf, err := loadConfigFile(path)
	if err != nil {
		fmt.Printf("%s[!] Profile registry unreadable (refusing to overwrite): %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}
	cf.Profiles[args[0]] = Profile{Name: args[0], Target: args[1], Token: authToken}
	if cf.CurrentContext == "" {
		cf.CurrentContext = args[0]
	}
	if err := saveConfigFile(path, cf); err != nil {
		fmt.Printf("%s[!] Could not persist profile: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}
	fmt.Printf("%s[+] Profile '%s' saved.%s\n", cGreen, args[0], cReset)
}

func runUseProfile(cmd *cobra.Command, args []string) {
	path := getCfgPath()
	cf, err := loadConfigFile(path)
	if err != nil {
		fmt.Printf("%s[!] Profile registry unreadable: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}
	if _, ok := cf.Profiles[args[0]]; !ok {
		fmt.Printf("%s[!] Profile not found.%s\n", cRed, cReset)
		os.Exit(1)
	}
	cf.CurrentContext = args[0]
	if err := saveConfigFile(path, cf); err != nil {
		fmt.Printf("%s[!] Could not persist active context: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}
	fmt.Printf("%s[+] Switched to '%s'.%s\n", cGreen, args[0], cReset)
}

// History & Reporting Logic
func runHistory(cmd *cobra.Command, args []string) {
	entries, err := loadHistory()
	if err != nil {
		fmt.Printf("%s[!] History file unreadable: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}

	if outputJSON {
		if err := json.NewEncoder(os.Stdout).Encode(entries); err != nil {
			fmt.Fprintf(os.Stderr, "[!] JSON encode failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, cBold+"DATE\tPROFILE\tTARGET\tRESULT"+cReset)

	limit := 0
	if len(entries) > 15 {
		limit = len(entries) - 15
	}
	for i := len(entries) - 1; i >= limit; i-- {
		e := entries[i]
		resColor := cGreen
		if e.Result == "FAILED" {
			resColor = cRed
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s%s%s\n", e.Timestamp.Format("01-02 15:04"), e.Profile, e.Target, resColor, e.Result, cReset)
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] output flush failed: %v\n", err)
	}
}

func runReport(cmd *cobra.Command, args []string) {
	entries, err := loadHistory()
	if err != nil {
		fmt.Printf("%s[!] History file unreadable: %v%s\n", cRed, err, cReset)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Printf("%s[!] No strikes recorded yet.%s\n", cYellow, cReset)
		return
	}
	last := entries[len(entries)-1]

	report := fmt.Sprintf("# Chaos Post-Mortem\n**Date:** %s\n**Target:** %s\n**Status:** %s\n",
		last.Timestamp.Format(time.RFC1123), last.Target, last.Result)

	fileName := fmt.Sprintf("report_%d.md", last.Timestamp.Unix())
	if err := os.WriteFile(fileName, []byte(report), 0o644); err != nil {
		fmt.Printf("%s[!] Report write failed (%s): %v%s\n", cRed, fileName, err, cReset)
		os.Exit(1)
	}
	fmt.Printf("%s[+] Report generated: %s%s\n", cGreen, fileName, cReset)
}

// Global Helpers
func getCfgPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] could not resolve home dir, falling back to cwd: %v\n", err)
		home = "."
	}
	return filepath.Join(home, ".pastaayctl.json")
}

func loadConfigFile(path string) (ConfigFile, error) {
	cf := ConfigFile{Profiles: make(map[string]Profile)}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cf, nil
		}
		return cf, fmt.Errorf("read %s: %w", path, err)
	}

	if len(data) == 0 {
		return cf, nil
	}

	if err := json.Unmarshal(data, &cf); err != nil {
		return cf, fmt.Errorf("parse %s (corrupt registry?): %w", path, err)
	}

	if cf.Profiles == nil {
		cf.Profiles = make(map[string]Profile)
	}
	return cf, nil
}

func saveConfigFile(path string, cf ConfigFile) error {
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func loadHistory() ([]StrikeEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	historyPath := filepath.Join(home, ".pastaay_history.json")

	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", historyPath, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var entries []StrikeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse %s (corrupt history?): %w", historyPath, err)
	}
	return entries, nil
}

func recordStrike(target, sType, result string) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	historyPath := filepath.Join(home, ".pastaay_history.json")

	cf, cfErr := loadConfigFile(getCfgPath())
	if cfErr != nil {
		fmt.Fprintf(os.Stderr, "[!] profile registry read failed during strike record: %v\n", cfErr)
	}

	var entries []StrikeEntry
	if data, err := os.ReadFile(historyPath); err == nil && len(data) > 0 {
		if uerr := json.Unmarshal(data, &entries); uerr != nil {
			backup := historyPath + ".corrupt"
			if werr := os.WriteFile(backup, data, 0o600); werr != nil {
				fmt.Fprintf(os.Stderr, "[!] history corrupt AND backup failed (%s): %v\n", backup, werr)
			} else {
				fmt.Fprintf(os.Stderr, "[!] history corrupt, backed up to %s: %v\n", backup, uerr)
			}
			entries = nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[!] history read failed: %v\n", err)
		return
	}

	entries = append(entries, StrikeEntry{
		Timestamp: time.Now(),
		Profile:   cf.CurrentContext,
		Target:    target,
		Type:      sType,
		Result:    result,
	})

	newData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] history marshal failed: %v\n", err)
		return
	}
	if err := os.WriteFile(historyPath, newData, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "[!] history write failed: %v\n", err)
	}
}
