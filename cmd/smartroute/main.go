package main

import (
	"fmt"
	"os"

	"github.com/bslie/smartroute/internal/cli"
)

var version = "0.1.0"

func main() {
	cli.RootCmd.Version = version
	if err := cli.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
