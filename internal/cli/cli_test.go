package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/config"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

func TestParseModes(t *testing.T) {
	f, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.Mode != ModeOnce {
		t.Errorf("default mode = %q, want once", f.Mode)
	}

	f, err = Parse([]string{"--watch", "--interval", "10s", "--count", "3"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Mode != ModeWatch || f.Interval != 10*time.Second || f.Count != 3 {
		t.Errorf("watch flags not parsed: %+v", f)
	}
}

// An endpoint list replaces the built-in one rather than adding to it, so the
// flag has to be repeatable.
func TestParseRepeatedEndpoints(t *testing.T) {
	f, err := Parse([]string{"--endpoint", "https://a.invalid", "--endpoint", "https://b.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Endpoints) != 2 {
		t.Errorf("Endpoints = %v, want both", f.Endpoints)
	}
}

func TestParseRejectsStrayArguments(t *testing.T) {
	if _, err := Parse([]string{"speedtest"}); err == nil {
		t.Error("a stray argument must be reported rather than ignored")
	}
}

// Given distinguishes a flag the user gave from a zero value, which is what
// lets the command line override a configuration file only when it really
// said something.
func TestGivenTracksWhatWasTyped(t *testing.T) {
	f, _ := Parse([]string{"--retries", "0"})
	if !f.Given("retries") {
		t.Error("an explicit --retries 0 must register as given")
	}
	if f.Given("timeout") {
		t.Error("an untyped flag must not register as given")
	}
}

func TestQuickImpliesOneSample(t *testing.T) {
	f, _ := Parse([]string{"--quick"})
	if f.Servers != quickSamples || !f.Given("servers") {
		t.Errorf("--quick did not settle the sample count: %+v", f)
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]struct {
		args    []string
		wantErr bool
	}{
		"plain run":                 {nil, false},
		"watch with interval":       {[]string{"--watch", "--interval", "10s"}, false},
		"interval without watch":    {[]string{"--interval", "10s"}, true},
		"count without watch":       {[]string{"--count", "2"}, true},
		"interval too short":        {[]string{"--watch", "--interval", "100ms"}, true},
		"nothing left to measure":   {[]string{"--no-download", "--no-upload", "--no-layers"}, true},
		"two of three disabled":     {[]string{"--no-download", "--no-upload"}, false},
		"unsupported language":      {[]string{"--lang", "fr"}, true},
		"supported language":        {[]string{"--lang", "zh-TW"}, false},
		"contradictory history":     {[]string{"--no-history", "--history", "x.jsonl"}, true},
		"zero servers":              {[]string{"--servers", "0"}, true},
		"negative retries rejected": {[]string{"--retries", "-2"}, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, err := Parse(c.args)
			if err != nil {
				if !c.wantErr {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			if err := f.Validate(); (err != nil) != c.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

// The exit codes are a contract with scripts and schedulers.
func TestClassify(t *testing.T) {
	cases := map[string]struct {
		report engine.Report
		err    error
		want   ExitCode
	}{
		"clean run": {engine.Report{}, nil, ExitOK},
		"partial":   {engine.Report{Errors: []string{"dns: timeout"}}, nil, ExitPartial},
		"no network": {
			engine.Report{}, engine.ErrNoNetwork, ExitNoNetwork,
		},
		"interrupted": {engine.Report{}, context.Canceled, ExitInterrupted},
		"every endpoint failed": {
			engine.Report{
				Download: throughput.Result{Err: throughput.ErrNoEndpoints},
				Upload:   throughput.Result{Err: throughput.ErrNoEndpoints},
				Errors:   []string{"download: no endpoint configured"},
			},
			nil, ExitNoEndpoint,
		},
		"throughput succeeded": {
			engine.Report{Download: throughput.Result{OK: true, Mbps: 100}}, nil, ExitOK,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := classify(c.report, c.err); got != c.want {
				t.Errorf("classify() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"--help", "--version"} {
		var stdout, stderr bytes.Buffer
		code, err := Run(context.Background(), []string{arg}, &stdout, &stderr)
		if err != nil || code != ExitOK {
			t.Errorf("%s: code %d, err %v", arg, code, err)
		}
		if stdout.Len() == 0 {
			t.Errorf("%s: printed nothing", arg)
		}
	}
}

func TestRunRejectsContradictoryFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Run(context.Background(), []string{"--interval", "5s"}, &stdout, &stderr)
	if code != ExitConfig {
		t.Errorf("code = %d, want ExitConfig", code)
	}
	if err == nil {
		t.Error("err = nil; the user must be told what was wrong")
	}
}

// The usage text is what a user reads when they are already confused, so it has
// to name every mode and the exit codes scripts depend on.
func TestUsageCoversTheContract(t *testing.T) {
	text := usage()
	for _, want := range []string{"--watch", "--json", "--endpoint", "--icmp", "Exit codes", webhookEnvName} {
		if !strings.Contains(text, want) {
			t.Errorf("usage omits %q", want)
		}
	}
	// The webhook is a credential; the help must steer it away from the shell
	// history a command line lands in.
	if !strings.Contains(text, "shell history") {
		t.Error("usage does not warn that a webhook on the command line is recorded")
	}
}

func TestMergePrefersFlagsOverFile(t *testing.T) {
	f, _ := Parse([]string{"--timeout", "7s", "--lang", "zh-TW"})
	cfg := merge(defaultsForTest(), f)

	if cfg.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v, want the flag value", cfg.Timeout)
	}
	if cfg.Language != "zh-TW" {
		t.Errorf("Language = %q, want the flag value", cfg.Language)
	}
	// An untyped flag must leave the configured value alone.
	if cfg.Interval != 30*time.Second {
		t.Errorf("Interval = %v, want the configured value", cfg.Interval)
	}
}

func TestEngineOptionsCarryOverrides(t *testing.T) {
	f, _ := Parse([]string{"--layer2", "203.0.113.53", "--icmp", "--no-upload"})
	opts := engineOptions(merge(defaultsForTest(), f), f)

	if opts.LayerOverrides.RegionalEgress != "203.0.113.53" {
		t.Errorf("layer override lost: %+v", opts.LayerOverrides)
	}
	if !opts.SkipUpload {
		t.Error("--no-upload did not reach the engine")
	}
	if opts.LatencyMethod == "" {
		t.Error("--icmp did not reach the engine")
	}
}

func TestOpenHistoryRespectsNoHistory(t *testing.T) {
	cfg := defaultsForTest()
	cfg.NoHistory = true
	store, err := openHistory(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if store != nil {
		t.Error("a store was opened despite --no-history")
	}
}

func TestErrNoNetworkSurvivesWrapping(t *testing.T) {
	if !errors.Is(engine.ErrNoNetwork, engine.ErrNoNetwork) {
		t.Fatal("sanity")
	}
	if classify(engine.Report{}, errors.Join(engine.ErrNoNetwork)) != ExitNoNetwork {
		t.Error("a wrapped ErrNoNetwork must still classify as no network")
	}
}

// defaultsForTest stands in for a machine with no configuration file.
func defaultsForTest() config.Config { return config.Defaults() }
