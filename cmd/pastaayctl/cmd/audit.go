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
	cf := loadConfigFile(getCfgPath())

	if outputJSON {
		json.NewEncoder(os.Stdout).Encode(cf.Profiles)
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
	w.Flush()
}

func runAddProfile(cmd *cobra.Command, args []string) {
	path := getCfgPath()
	cf := loadConfigFile(path)
	cf.Profiles[args[0]] = Profile{Name: args[0], Target: args[1], Token: authToken}
	if cf.CurrentContext == "" {
		cf.CurrentContext = args[0]
	}
	saveConfigFile(path, cf)
	fmt.Printf("%s[+] Profile '%s' saved.%s\n", cGreen, args[0], cReset)
}

func runUseProfile(cmd *cobra.Command, args []string) {
	path := getCfgPath()
	cf := loadConfigFile(path)
	if _, ok := cf.Profiles[args[0]]; !ok {
		fmt.Printf("%s[!] Profile not found.%s\n", cRed, cReset)
		return
	}
	cf.CurrentContext = args[0]
	saveConfigFile(path, cf)
	fmt.Printf("%s[+] Switched to '%s'.%s\n", cGreen, args[0], cReset)
}

// History & Reporting Logic

func runHistory(cmd *cobra.Command, args []string) {
	entries := loadHistory()

	if outputJSON {
		json.NewEncoder(os.Stdout).Encode(entries)
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
	w.Flush()
}

func runReport(cmd *cobra.Command, args []string) {
	entries := loadHistory()
	if len(entries) == 0 {
		return
	}
	last := entries[len(entries)-1]

	report := fmt.Sprintf("# Chaos Post-Mortem\n**Date:** %s\n**Target:** %s\n**Status:** %s\n",
		last.Timestamp.Format(time.RFC1123), last.Target, last.Result)

	fileName := fmt.Sprintf("report_%d.md", last.Timestamp.Unix())
	os.WriteFile(fileName, []byte(report), 0644)
	fmt.Printf("%s[+] Report generated: %s%s\n", cGreen, fileName, cReset)
}

// Global Helpers

func getCfgPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pastaayctl.json")
}

func loadConfigFile(path string) ConfigFile {
	data, err := os.ReadFile(path)

	cf := ConfigFile{Profiles: make(map[string]Profile)}

	if err == nil {
		json.Unmarshal(data, &cf)
	}
	
	if cf.Profiles == nil {
		cf.Profiles = make(map[string]Profile)
	}
	return cf
}

func saveConfigFile(path string, cf ConfigFile) {
	data, _ := json.MarshalIndent(cf, "", "  ")
	os.WriteFile(path, data, 0600)
}

func loadHistory() []StrikeEntry {
	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, ".pastaay_history.json"))
	var entries []StrikeEntry
	json.Unmarshal(data, &entries)
	return entries
}

func recordStrike(target, sType, result string) {
	home, _ := os.UserHomeDir()
	historyPath := filepath.Join(home, ".pastaay_history.json")
	cf := loadConfigFile(getCfgPath())

	var entries []StrikeEntry
	data, _ := os.ReadFile(historyPath)
	_ = json.Unmarshal(data, &entries)

	entries = append(entries, StrikeEntry{
		Timestamp: time.Now(),
		Profile:   cf.CurrentContext,
		Target:    target,
		Type:      sType,
		Result:    result,
	})

	newData, _ := json.MarshalIndent(entries, "", "  ")
	_ = os.WriteFile(historyPath, newData, 0644)
}
