package main

import (
	"fmt"
	"os"

	"github.com/hyqhyq3/mymtr/internal/cli"
	"github.com/hyqhyq3/mymtr/internal/i18n"
)

func main() {
	// Initialize i18n before creating commands (auto-detect system locale)
	i18n.Init("")

	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
