package cmd

import (
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
	Long:  cCyan + splash + cReset + "\n\n" + cGray + "Neural Link established for Pastaay Engine v2.0." + cReset,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() error {
	buildSeniorHelpTemplate()
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&targetURL, "target", "t", "", "Target engine URL")
	rootCmd.PersistentFlags().StringVarP(&authToken, "token", "k", "", "Auth token")
	rootCmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "", "Active context")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "JSON output mode")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		setupEnvironment()
	}
}

func buildSeniorHelpTemplate() {

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
		"lint": "guard", "plan": "guard", "validate": "guard", "autopilot": "guard",
		"audit": "system", "profile": "system", "generate": "system", "export": "system",
	}

	for _, cmd := range rootCmd.Commands() {
		if groupID, exists := cmdMap[cmd.Name()]; exists {
			cmd.GroupID = groupID
		}
	}

	helpTmpl := `{{.Long}}

` + cBold + cCyan + `USAGE` + cReset + `
  {{.UseLine}}
{{if .HasAvailableSubCommands}}{{range .Groups}}
{{.Title}}{{range $.Commands}}{{if eq .GroupID $.ID}}
  ` + cBold + `{{rpad .Name 15}}` + cReset + ` ` + cGray + `{{.Short}}` + cReset + `{{end}}{{end}}
{{end}}{{end}}
` + cBold + cCyan + `GLOBAL FLAGS` + cReset + `
{{.PersistentFlags.FlagUsages | trimTrailingWhitespaces}}
`
	rootCmd.SetHelpTemplate(helpTmpl)
}

func setupEnvironment() {

	configPath := getCfgPath()
	cf := loadConfigFile(configPath)
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
