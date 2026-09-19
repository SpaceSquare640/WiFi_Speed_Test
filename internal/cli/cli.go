// Package cli is the command line shell around the core engine.
//
// It may import core. Core may not import it.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/config"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/latency"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/layers"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/notify"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
	"github.com/SpaceSquare640/WiFi_Speed_Test/internal/cli/output"
)

// Version is the build identity, overridden at link time.
var Version = "dev"

// webhookEnvName is repeated in the flag help so that the credential has a
// visible home that is not the command line, where it would land in shell
// history.
const webhookEnvName = config.WebhookEnvVar

// ExitCode is the process status. The values are part of the tool's contract:
// the audience includes developers who will place it in scripts and schedulers,
// so they may not be renumbered casually.
type ExitCode int

const (
	// ExitOK reports that every measurement succeeded.
	ExitOK ExitCode = 0

	// ExitPartial reports that some measurements failed but a usable report was
	// produced.
	ExitPartial ExitCode = 1

	// ExitNoNetwork reports that no network connection was available.
	ExitNoNetwork ExitCode = 2

	// ExitNoEndpoint reports that every throughput endpoint failed.
	ExitNoEndpoint ExitCode = 3

	// ExitConfig reports a malformed configuration or contradictory flags.
	ExitConfig ExitCode = 4

	// ExitInterrupted reports that the user stopped the run.
	ExitInterrupted ExitCode = 130
)

// Run parses args, executes the requested mode and returns the process status.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) (ExitCode, error) {
	flags, err := Parse(args)
	if err != nil {
		fmt.Fprintln(stderr, usage())
		return ExitConfig, err
	}
	if flags.Help {
		fmt.Fprintln(stdout, usage())
		return ExitOK, nil
	}
	if flags.Version {
		fmt.Fprintf(stdout, "wifitest %s\n", Version)
		return ExitOK, nil
	}
	if err := flags.Validate(); err != nil {
		return ExitConfig, err
	}

	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		// A malformed configuration is not something to paper over: the user
		// wrote settings that are not being applied, and measuring with silent
		// defaults would answer a question they did not ask.
		return ExitConfig, fmt.Errorf("configuration: %w", err)
	}
	cfg = merge(cfg, flags)

	store, err := openHistory(cfg)
	if err != nil {
		// History is a convenience. Losing it must not cost the measurement.
		fmt.Fprintf(stderr, "history unavailable: %v\n", err)
		store = nil
	}
	if store != nil {
		defer store.Close()
	}

	writer := output.New(flags.JSON, output.Options{
		Out:   stdout,
		Color: useColour(cfg, flags, stdout),
		Lang:  cfg.Language,
		Watch: flags.Mode == ModeWatch,
	})
	defer writer.Close()

	runner := &Runner{
		engine:   engine.New(engineOptions(cfg, flags)),
		store:    store,
		notifier: notify.New(notify.Options{URL: cfg.WebhookURL, Format: notify.FormatDiscord, Retries: cfg.Retries}),
		writer:   writer,
		stderr:   stderr,
		interval: cfg.Interval,
		count:    flags.Count,
	}

	if flags.Mode == ModeWatch {
		return runner.RunWatch(ctx)
	}
	return runner.RunOnce(ctx)
}

// merge layers the command line over the configuration file, which in turn sits
// over the built-in defaults.
func merge(cfg config.Config, f Flags) config.Config {
	if len(f.Endpoints) > 0 {
		// A user-supplied list replaces the configured one outright; mixing the
		// two would average across unlike conditions.
		cfg.Endpoints = f.Endpoints
	}
	if f.Layer1 != "" {
		cfg.Layer1 = f.Layer1
	}
	if f.Layer2 != "" {
		cfg.Layer2 = f.Layer2
	}
	if f.Layer3 != "" {
		cfg.Layer3 = f.Layer3
	}
	if f.Given("interval") {
		cfg.Interval = f.Interval
	}
	if f.Given("timeout") {
		cfg.Timeout = f.Timeout
	}
	if f.Given("retries") {
		cfg.Retries = f.Retries
	}
	if f.Given("servers") {
		cfg.Samples = f.Servers
	}
	if f.WebhookURL != "" {
		cfg.WebhookURL = f.WebhookURL
	}
	if f.HistoryPath != "" {
		cfg.HistoryPath = f.HistoryPath
	}
	if f.NoHistory {
		cfg.NoHistory = true
	}
	if f.Lang != "" {
		cfg.Language = f.Lang
	}
	if f.NoColor {
		cfg.NoColor = true
	}
	return cfg
}

func engineOptions(cfg config.Config, f Flags) engine.Options {
	opts := engine.Options{
		SkipDownload: f.NoDownload,
		SkipUpload:   f.NoUpload,
		SkipLayers:   f.NoLayers,
		LayerOverrides: layers.Overrides{
			Gateway:        cfg.Layer1,
			RegionalEgress: cfg.Layer2,
			International:  cfg.Layer3,
		},
		Samples: cfg.Samples,
		Timeout: cfg.Timeout,
		Retries: cfg.Retries,
	}
	if f.ICMP {
		opts.LatencyMethod = latency.MethodICMP
	}
	for _, url := range cfg.Endpoints {
		// A bare URL configures both directions; an endpoint that refuses
		// uploads reports that itself when the upload is attempted.
		opts.Endpoints = append(opts.Endpoints, throughput.Endpoint{
			Name:        url,
			DownloadURL: url,
			UploadURL:   url,
		})
	}
	return opts
}

func openHistory(cfg config.Config) (history.Store, error) {
	if cfg.NoHistory {
		return nil, nil
	}
	path := cfg.HistoryPath
	if path == "" {
		var err error
		if path, err = history.DefaultPath(); err != nil {
			return nil, err
		}
	}
	return history.Open(path, history.DefaultLimit)
}

// useColour decides whether to emit escape sequences.
//
// Colour is for a person at a terminal. Redirected into a file or a pipe it is
// noise that breaks grep, so it is off unless the destination is a terminal.
func useColour(cfg config.Config, f Flags, stdout io.Writer) bool {
	if cfg.NoColor || f.JSON {
		return false
	}
	file, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// classify turns a report into the process status.
func classify(r engine.Report, err error) ExitCode {
	switch {
	case errors.Is(err, context.Canceled):
		return ExitInterrupted
	case errors.Is(err, engine.ErrNoNetwork), errors.Is(err, engine.ErrICMPBlocked):
		// Both mean nothing was measured. They are kept apart only so the
		// message can name the likelier cause, not to grade the outcome
		// differently.
		return ExitNoNetwork
	case err != nil:
		return ExitPartial
	}

	// Every endpoint failing is its own status: the network is up, the tool
	// simply had nowhere to measure against, and the fix is a different
	// endpoint rather than a different network.
	if endpointsExhausted(r) {
		return ExitNoEndpoint
	}
	if len(r.Errors) > 0 {
		return ExitPartial
	}
	return ExitOK
}

func endpointsExhausted(r engine.Report) bool {
	downloadFailed := r.Download.Err != nil
	uploadFailed := r.Upload.Err != nil
	if !downloadFailed && !uploadFailed {
		return false
	}
	// A skipped direction leaves a zero-valued result with no error, so only
	// genuine failures reach here.
	return !r.Download.OK && !r.Upload.OK
}

func usage() string {
	return `wifitest ` + Version + ` - network diagnostics that say which segment is at fault

Usage:
  wifitest [flags]

Modes:
  --watch                 repeat until stopped
  --interval <duration>   delay between passes (default 30s, watch only)
  --count <n>             stop after n passes (watch only)

Output:
  --json                  machine-readable output, one object per pass
  --no-color              disable colour
  --lang <en|zh-TW>       interface language

Scope:
  --quick                 sample a single endpoint
  --no-download           skip the download measurement
  --no-upload             skip the upload measurement
  --no-layers             skip the layered diagnostics

Targets:
  --endpoint <url>        throughput endpoint; repeat to supply several
                          (replaces the built-in list rather than adding to it)
  --layer1 <host>         local gateway target
  --layer2 <host>         regional egress target
  --layer3 <host>         international target

Measurement:
  --servers <n>           endpoints contributing to the mean
  --timeout <duration>    per-measurement timeout
  --retries <n>           retry attempts per measurement
  --icmp                  probe with ICMP rather than TCP (may need elevation)

Files:
  --config <path>         configuration file
  --history <path>        history file
  --no-history            do not record this run
  --webhook <url>         webhook to notify; prefer ` + webhookEnvName + `,
                          since a command line lands in shell history

Exit codes:
  0 success   1 partial   2 no network   3 no endpoint   4 config   130 interrupted`
}
