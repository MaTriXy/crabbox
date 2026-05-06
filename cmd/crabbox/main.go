package main

import (
	"context"
	"fmt"
	"os"

	"github.com/openclaw/crabbox/internal/cli"
	_ "github.com/openclaw/crabbox/internal/providers/all"
)

func main() {
	if err := cli.Run(context.Background(), os.Args[1:]); err != nil {
		var exit cli.ExitError
		if cli.AsExitError(err, &exit) {
			if exit.Message != "" {
				fmt.Fprintln(os.Stderr, exit.Message)
			}
			os.Exit(exit.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
