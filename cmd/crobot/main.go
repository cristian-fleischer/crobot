// Package main is the entry point for the crobot CLI.
package main

import (
	"fmt"
	"os"

	"github.com/cristian-fleischer/crobot/internal/cli"
)

func main() {
	cmd := cli.RootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
