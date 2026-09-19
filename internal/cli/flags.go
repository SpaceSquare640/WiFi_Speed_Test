package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// Mode selects between a single pass and continuous monitoring.
type Mode string

const (
	ModeOnce  Mode = "once"
	ModeWatch Mode = "watch"
)

// quickSamples is how many endpoints --quick settles for. The flag exists for
// someone who wants an answer now, and sampling one endpoint is the difference
// between a few seconds and a minute.
const quickSamples = 1

// maxStreams bounds --streams. Beyond a handful the extra connections stop
// revealing anything about the link and start describing what the endpoint
// tolerates.
const maxStreams = 16

// Flags is the parsed command line.
type Flags struct {
	Mode     Mode
	Interval time.Duration
	Count    int

	JSON    bool
	NoColor bool
	Lang    string

	Quick      bool
	NoDownload bool
	NoUpload   bool
	NoLayers   bool

	Endpoints         []string
	DownloadEndpoints []string
	UploadEndpoints   []string
	Layer1            string
	Layer2            string
	Layer3            string

	Servers int
	Streams int
	Timeout time.Duration
	Retries int
	ICMP    bool

	ConfigPath  string
	HistoryPath string
	NoHistory   bool
	WebhookURL  string

	Help    bool
	Version bool

	// set records which flags the user actually gave, so that configuration
	// file values are only overridden when the command line really said so. A
	// zero value cannot carry that distinction on its own.
	set map[string]bool
}

// Given reports whether the named flag appeared on the command line.
func (f Flags) Given(name string) bool { return f.set[name] }

// repeatable collects a flag that may be given more than once.
type repeatable []string

func (r *repeatable) String() string { return strings.Join(*r, ",") }

func (r *repeatable) Set(v string) error {
	if v == "" {
		return errors.New("empty value")
	}
	*r = append(*r, v)
	return nil
}

// Parse reads args into Flags.
func Parse(args []string) (Flags, error) {
	var f Flags
	var watch bool
	var endpoints, downloadEndpoints, uploadEndpoints repeatable

	fs := flag.NewFlagSet("wifitest", flag.ContinueOnError)
	// Usage is written by Run, which knows the output stream and the language.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.BoolVar(&watch, "watch", false, "repeat the diagnostic at a fixed interval")
	fs.DurationVar(&f.Interval, "interval", 0, "delay between passes in watch mode")
	fs.IntVar(&f.Count, "count", 0, "stop after this many passes in watch mode")

	fs.BoolVar(&f.JSON, "json", false, "emit machine-readable output")
	fs.BoolVar(&f.NoColor, "no-color", false, "disable colour")
	fs.StringVar(&f.Lang, "lang", "", "interface language: en or zh-TW")

	fs.BoolVar(&f.Quick, "quick", false, "sample a single endpoint")
	fs.BoolVar(&f.NoDownload, "no-download", false, "skip the download measurement")
	fs.BoolVar(&f.NoUpload, "no-upload", false, "skip the upload measurement")
	fs.BoolVar(&f.NoLayers, "no-layers", false, "skip the layered diagnostics")

	fs.Var(&endpoints, "endpoint", "throughput endpoint; repeat to supply several")
	fs.Var(&downloadEndpoints, "download-endpoint", "endpoint for downloads only; repeat to supply several")
	fs.Var(&uploadEndpoints, "upload-endpoint", "endpoint for uploads only; repeat to supply several")
	fs.StringVar(&f.Layer1, "layer1", "", "override the local gateway target")
	fs.StringVar(&f.Layer2, "layer2", "", "override the regional egress target")
	fs.StringVar(&f.Layer3, "layer3", "", "override the international target")

	fs.IntVar(&f.Servers, "servers", 0, "how many endpoints contribute to the mean")
	fs.IntVar(&f.Streams, "streams", 0, "parallel connections per throughput endpoint")
	fs.DurationVar(&f.Timeout, "timeout", 0, "per-measurement timeout")
	fs.IntVar(&f.Retries, "retries", -1, "retry attempts per measurement")
	fs.BoolVar(&f.ICMP, "icmp", false, "probe with ICMP instead of TCP (may need elevation)")

	fs.StringVar(&f.ConfigPath, "config", "", "configuration file path")
	fs.StringVar(&f.HistoryPath, "history", "", "history file path")
	fs.BoolVar(&f.NoHistory, "no-history", false, "do not record this run")
	fs.StringVar(&f.WebhookURL, "webhook", "", "webhook URL to notify (a credential; prefer the config file or "+webhookEnvName+")")

	fs.BoolVar(&f.Help, "help", false, "show usage")
	fs.BoolVar(&f.Version, "version", false, "show version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			f.Help = true
			return f, nil
		}
		return f, err
	}
	if fs.NArg() > 0 {
		return f, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	f.Endpoints = endpoints
	f.DownloadEndpoints = downloadEndpoints
	f.UploadEndpoints = uploadEndpoints
	f.Mode = ModeOnce
	if watch {
		f.Mode = ModeWatch
	}

	f.set = make(map[string]bool)
	fs.Visit(func(fl *flag.Flag) { f.set[fl.Name] = true })
	if f.Quick {
		f.Servers = quickSamples
		f.set["servers"] = true
	}
	return f, nil
}

// Validate rejects contradictory combinations, such as --interval without
// --watch, or disabling every measurement at once.
func (f Flags) Validate() error {
	if f.Mode != ModeWatch {
		if f.Given("interval") {
			return errors.New("--interval applies to --watch only")
		}
		if f.Given("count") {
			return errors.New("--count applies to --watch only")
		}
	}
	if f.NoDownload && f.NoUpload && f.NoLayers {
		// Every measurement disabled would leave a pass with nothing to do but
		// print a heading.
		return errors.New("--no-download, --no-upload and --no-layers together leave nothing to measure")
	}
	if f.Given("interval") && f.Interval < time.Second {
		return errors.New("--interval must be at least 1s")
	}
	if f.Given("count") && f.Count < 1 {
		return errors.New("--count must be at least 1")
	}
	if f.Given("servers") && f.Servers < 1 {
		return errors.New("--servers must be at least 1")
	}
	// Capped as well as floored. Parallel connections are a measurement
	// technique, not a throttle to open at will: a large number aimed at
	// someone else's server is indistinguishable from an attempt to flood it,
	// and this tool has no business shipping that as a one-word flag.
	if f.Given("streams") && (f.Streams < 1 || f.Streams > maxStreams) {
		return fmt.Errorf("--streams must be between 1 and %d", maxStreams)
	}
	if f.Given("timeout") && f.Timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	if f.Given("retries") && f.Retries < 0 {
		return errors.New("--retries cannot be negative")
	}
	if f.Given("lang") && !supportedLang(f.Lang) {
		return fmt.Errorf("unsupported language %q: use en or zh-TW", f.Lang)
	}
	if f.Given("no-history") && f.Given("history") {
		return errors.New("--history and --no-history contradict each other")
	}
	return nil
}

func supportedLang(lang string) bool {
	normalised := strings.ToLower(strings.ReplaceAll(lang, "_", "-"))
	return strings.HasPrefix(normalised, "en") || strings.HasPrefix(normalised, "zh")
}
