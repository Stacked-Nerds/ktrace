package main

import (
	"os"

	"github.com/Stacked-Nerds/ktrace/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
