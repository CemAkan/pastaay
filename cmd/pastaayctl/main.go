package main

import (
	"os"

	"github.com/CemAkan/pastaay/cmd/pastaayctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
