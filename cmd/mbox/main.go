package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/mlhiter/mbox/internal/cli"
)

func main() {
	app := cli.NewApp(cli.Streams{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
