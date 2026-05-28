package cmd

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	targetURL   string
	authToken   string
	outputJSON  bool
	profileName string
)

const (
	cReset  = "\033[0m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cBold   = "\033[1m"
	cGray   = "\033[90m"
	cPurple = "\033[35m"
	cWhite  = "\033[37m"
)

const splash = `
   ██████╗  █████╗ ███████╗████████╗ █████╗  █████╗ ██╗   ██╗
   ██╔══██╗██╔══██╗██╔════╝╚══██╔══╝██╔══██╗██╔══██╗╚██╗ ██╔╝
   ██████╔╝███████║███████╗   ██║   ███████║███████║ ╚████╔╝ 
   ██╔═══╝ ██╔══██║╚════██║   ██║   ██╔══██║██╔══██║  ╚██╔╝  
   ██║     ██║  ██║███████║   ██║   ██║  ██║██║  ██║   ██║   
   ╚═╝     ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   `

var rootCmd = &cobra.Command{
	Use:   "pastaayctl",
	Short: "Enterprise Chaos Orchestrator",

	Long: cCyan + splash + cReset + "\n\n" + cGray + "Neural Link established for Pastaay Engine v2.0.\n\"Nature doesn't recognize good and evil... only balance and imbalance.\"" + cReset,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() error {
	buildCustomHelpTemplate()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&targetURL, "target", "t", "", "Target engine URL")
	rootCmd.PersistentFlags().StringVarP(&authToken, "token", "k", "", "Auth token")
	rootCmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "", "Active context")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "JSON output mode")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		setupEnvironment()
		triggerWhiteTulipAnomaly(cmd.Context())
	}
}

func buildCustomHelpTemplate() {
	groups := []struct {
		ID    string
		Title string
	}{
		{"attack", cBold + cRed + "ATTACK (Injections)" + cReset},
		{"view", cBold + cGreen + "OBSERVABILITY (View)" + cReset},
		{"guard", cBold + cYellow + "GUARD (Safety & Analysis)" + cReset},
		{"system", cBold + cPurple + "SYSTEM (Management)" + cReset},
	}

	for _, g := range groups {
		rootCmd.AddGroup(&cobra.Group{ID: g.ID, Title: g.Title})
	}

	cmdMap := map[string]string{
		"strike": "attack", "inject": "attack", "snipe": "attack", "rollback": "attack", "broadcast": "attack", "run": "attack",
		"top": "view", "status": "view", "discover": "view", "inspect": "view",
		"lint": "guard", "plan": "guard", "validate": "guard", "autopilot": "guard", "oracle": "guard",
		"audit": "system", "profile": "system", "util": "system",
	}

	for _, cmd := range rootCmd.Commands() {
		if groupID, exists := cmdMap[cmd.Name()]; exists {
			cmd.GroupID = groupID
		}
	}

	helpTmpl := `{{.Long}}

` + cBold + cCyan + `USAGE` + cReset + `
  {{.UseLine}}
{{if .HasAvailableSubCommands}}{{range $group := .Groups}}
{{$group.Title}}{{range $cmd := $.Commands}}{{if eq $cmd.GroupID $group.ID}}
  ` + cBold + `{{rpad $cmd.Name 15}}` + cReset + ` ` + cGray + `{{$cmd.Short}}` + cReset + `{{end}}{{end}}
{{end}}{{end}}
` + cBold + cCyan + `GLOBAL FLAGS` + cReset + `
{{.PersistentFlags.FlagUsages | trimTrailingWhitespaces}}
`
	rootCmd.SetHelpTemplate(helpTmpl)
}

func setupEnvironment() {

	configPath := getCfgPath()
	cf, err := loadConfigFile(configPath)

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot load profile registry (%s): %v\n", configPath, err)
		os.Exit(1)
	}

	active := cf.CurrentContext
	if profileName != "" {
		active = profileName
	}
	if p, ok := cf.Profiles[active]; ok {
		if targetURL == "" {
			targetURL = p.Target
		}
		if authToken == "" {
			authToken = p.Token
		}
	}
	if targetURL == "" {
		targetURL = "http://localhost:2112/metrics"
	}
}

func triggerWhiteTulipAnomaly(ctx context.Context) {
	if rand.Float64() < 0.01 {
		tulip := "              \n" +
			"     /\\^/`\\   \n" +
			"    | \\/   |  \n" +
			"    | |    |  \n" +
			"    \\ \\    /  \n" +
			"     '\\\\//'   \n" +
			"       ||     \n" +
			"       ||     \n" +
			"       ||     \n" +
			"       ||  ,  \n" +
			"   |\\  ||  |\\ \n" +
			"   | | ||  | |\n" +
			"   | | || / / \n" +
			"    \\ \\||/ /  \n" +
			"     `\\\\//`   \n" +
			"    ^^^^^^^^  "

		fmt.Print("\033[H\033[2J")
		fmt.Printf("\n%s[SYSTEM ANOMALY DETECTED]%s\n", cBold+cRed, cReset)
		fmt.Printf("%s%s%s\n\n", cBold+cWhite, tulip, cReset)
		fmt.Printf("%s\"I asked God for a sign of forgiveness. A specific one. A white tulip.\"\n - W.B.%s\n\n", cGray, cReset)

		select {
		case <-ctx.Done():
		case <-time.After(7 * time.Second):
		}
		fmt.Print("\033[H\033[2J")
	}
}
