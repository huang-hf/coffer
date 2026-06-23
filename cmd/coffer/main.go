package main

import (
	"os"

	"github.com/huang-hf/coffer/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
