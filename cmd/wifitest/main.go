// Command wifitest measures network throughput and diagnoses which segment of
// the path is responsible when something is wrong.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/SpaceSquare640/WiFi_Speed_Test/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	code, err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(int(code))
}
