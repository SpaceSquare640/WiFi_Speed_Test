// Command wifitest measures network throughput and diagnoses which segment of
// the path is responsible when something is wrong.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/SpaceSquare640/WiFi_Speed_Test/internal/cli"
)

// debugArgsEnv dumps the raw command line and exits.
//
// It is an environment variable rather than a flag on purpose: the failures it
// exists for are ones where no flag can be read at all, so a flag to diagnose
// them would die alongside everything else.
const debugArgsEnv = "WIFITEST_DEBUG_ARGS"

func main() {
	if os.Getenv(debugArgsEnv) != "" {
		dumpArgs(os.Stdout)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Not os.Args[1:] — see cli.Args, which corrects for the loader Termux has
	// to invoke this through.
	code, err := cli.Run(ctx, cli.Args(), os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(int(code))
}

// dumpArgs prints what this process was actually handed, which on a platform
// that loads the program rather than exec'ing it is not what the command line
// appeared to say.
func dumpArgs(w io.Writer) {
	exe, err := os.Executable()
	fmt.Fprintf(w, "executable: %q (err: %v)\n", exe, err)
	fmt.Fprintf(w, "argc: %d\n", len(os.Args))
	for i, arg := range os.Args {
		fmt.Fprintf(w, "argv[%d]: %q\n", i, arg)
	}
	fmt.Fprintf(w, "parsed as: %q\n", cli.Args())
}
