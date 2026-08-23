package main

import (
	"fmt"
	"os"

	"github.com/vecyang1/appsumo-cli/internal/cli"
)

func main() {
	if err := cli.NewRoot(cli.Options{}).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
