package main

import (
	"fmt"
	"os"

	"github.com/yangqihuang/mymtr/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
