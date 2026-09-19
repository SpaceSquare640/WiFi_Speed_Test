// Package engine orchestrates one diagnostic pass and owns the report type that
// every shell renders.
//
// Nothing under core may import the cli tree. The desktop build consumes this
// engine through a sidecar binary and the Android build through a gomobile
// binding, so a dependency pointing the other way would strand both.
package engine

import (
	"context"
	"errors"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/dns"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/grade"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/latency"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/layers"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

// SchemaVersion identifies the shape of Report as serialised to JSON.
// Downstream parsers depend on it; raise it whenever the shape changes.
const SchemaVersion = 1

// LayerResult pairs a resolved layer with what probing it found.
type LayerResult struct {
	Layer  layers.Layer
	Result latency.Result
}

// Host describes where the run took place.
type Host struct {
	Name string
	OS   string
	Arch string
}

// Report is the complete outcome of one pass.
type Report struct {
	SchemaVersion int
	Timestamp     time.Time
	Host          Host

	DNS      dns.Result
	Layers   []LayerResult
	Download throughput.Result
	Upload   throughput.Result

	// Latency is measured against the throughput endpoints, distinct from the
	// per-layer figures.
	Latency latency.Result

	Grade grade.Grade

	// ResolverIsPublic carries the layer set's caveat into the report: the host
	// resolver is itself a public address, so the regional layer has collapsed
	// onto the international one and the two rows are not independent evidence.
	ResolverIsPublic bool

	// RegionalUnsupported carries the other caveat: the regional row is absent
	// because this platform keeps no resolver configuration, not because the
	// layer was probed and failed.
	RegionalUnsupported bool

	// Errors collects failures that did not abort the pass. A partial report is
	// more useful than none: a run that cannot reach the internet has still
	// established that the gateway responds.
	Errors []string
}

// Options configures a pass.
type Options struct {
	SkipDownload bool
	SkipUpload   bool
	SkipLayers   bool

	LayerOverrides layers.Overrides
	Endpoints      []throughput.Endpoint
	Samples        int

	// Streams is how many connections each throughput endpoint is measured
	// over. One describes a single connection; more describe several.
	Streams       int
	Timeout       time.Duration
	Retries       int
	LatencyMethod latency.Method
	DNSProbeHost  string
}

// Defaults applied to a zero-valued Options.
const (
	DefaultSamples = 3
	DefaultTimeout = 10 * time.Second
	DefaultRetries = 3

	// LatencyProbes is how many probes each diagnostic layer receives. Jitter
	// needs several samples to mean anything.
	LatencyProbes = 5

	// retryBackoff is the first retry delay; it doubles on each attempt. A
	// network that just failed is often a network still failing, so retrying
	// immediately would mostly reproduce the failure.
	retryBackoff = 500 * time.Millisecond
)

// ErrNoNetwork reports that nothing at all could be measured, which makes the
// pass meaningless rather than merely partial.
var ErrNoNetwork = errors.New("engine: no measurement succeeded; the host appears to have no network")

// ErrICMPBlocked replaces ErrNoNetwork when the pass probed with ICMP.
//
// ICMP is filtered by far more networks and hosts than TCP is, so a run that
// measures nothing under --icmp usually met a filter rather than a dead link —
// the TCP probe would very likely have answered. The tool does not silently
// retry over TCP, because a user who asked for ICMP is entitled to the ICMP
// answer; it says what happened and what to run instead.
var ErrICMPBlocked = errors.New("engine: nothing answered over ICMP, which is commonly blocked; rerun without --icmp to probe with TCP")

// Engine runs diagnostic passes.
type Engine struct {
	opts Options
}

// New returns an Engine bound to opts.
func New(opts Options) *Engine {
	if opts.Samples <= 0 {
		opts.Samples = DefaultSamples
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Retries < 0 {
		opts.Retries = DefaultRetries
	}
	if opts.LatencyMethod == "" {
		opts.LatencyMethod = latency.MethodTCP
	}
	if opts.DNSProbeHost == "" {
		opts.DNSProbeHost = dns.DefaultHost
	}
	return &Engine{opts: opts}
}

// Run performs one pass. It returns a Report even when parts failed; a non-nil
// error is reserved for conditions that made the pass meaningless.
//
// The measurements run in sequence rather than in parallel. Probes issued at the
// same time contend for the same link and distort each other, which would turn
// the layered comparison — the whole point of this tool — into noise.
func (e *Engine) Run(ctx context.Context) (Report, error) {
	report := Report{
		SchemaVersion: SchemaVersion,
		Timestamp:     time.Now(),
		Host:          hostInfo(),
	}

	if !e.opts.SkipLayers {
		e.runLayers(ctx, &report)
	}
	e.runDNS(ctx, &report)
	e.runLatency(ctx, &report)
	e.runThroughput(ctx, &report)

	// A grade is a judgement about a measurement. Without the measurement there
	// is no judgement to publish.
	if report.Download.OK {
		report.Grade = grade.Evaluate(report.Download.Mbps)
	} else {
		report.Grade = grade.GradeUnknown
	}

	if !anySucceeded(report) {
		if e.opts.LatencyMethod == latency.MethodICMP {
			return report, ErrICMPBlocked
		}
		return report, ErrNoNetwork
	}
	return report, nil
}

// runLayers resolves the diagnostic layers and probes each one.
func (e *Engine) runLayers(ctx context.Context, report *Report) {
	set, err := layers.Resolve(e.opts.LayerOverrides)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		return
	}
	report.ResolverIsPublic = set.ResolverIsPublic
	report.RegionalUnsupported = set.RegionalUnsupported

	for _, layer := range set.Layers {
		if ctx.Err() != nil {
			return
		}
		result := latency.Measure(ctx, layer.Target, latency.Options{
			Method:  e.opts.LatencyMethod,
			Count:   LatencyProbes,
			Timeout: e.opts.Timeout,
			Ports:   layer.ProbePorts(),
		})
		// A layer that does not answer is recorded, not discarded: an
		// unreachable gateway is the most informative result this tool can
		// produce, and dropping the row would hide it.
		if result.Err != nil {
			report.Errors = append(report.Errors, string(layer.Kind)+": "+result.Err.Error())
		}
		report.Layers = append(report.Layers, LayerResult{Layer: layer, Result: result})
	}
}

// runDNS measures name resolution.
func (e *Engine) runDNS(ctx context.Context, report *Report) {
	report.DNS = retry(ctx, e.opts.Retries, func() (dns.Result, bool) {
		r := dns.Measure(ctx, dns.Options{Host: e.opts.DNSProbeHost, Timeout: e.opts.Timeout})
		// An unsupported platform will not have become supported by the second
		// attempt, so retrying it only spends the backoff.
		return r, r.OK || errors.Is(r.Err, dns.ErrUnsupported)
	})
	// A platform that cannot be measured has not failed, so it does not belong
	// in the problem list; the renderer states it in its own row instead.
	if report.DNS.Err != nil && !errors.Is(report.DNS.Err, dns.ErrUnsupported) {
		report.Errors = append(report.Errors, "dns: "+report.DNS.Err.Error())
	}
}

// runThroughput measures both directions unless they were skipped.
func (e *Engine) runThroughput(ctx context.Context, report *Report) {
	if !e.opts.SkipDownload {
		report.Download = e.measureThroughput(ctx, throughput.DirectionDownload)
		if report.Download.Err != nil {
			report.Errors = append(report.Errors, "download: "+report.Download.Err.Error())
		}
	}
	if !e.opts.SkipUpload {
		report.Upload = e.measureThroughput(ctx, throughput.DirectionUpload)
		if report.Upload.Err != nil {
			report.Errors = append(report.Errors, "upload: "+report.Upload.Err.Error())
		}
	}
}

// endpoints returns the targets for this pass. A user-supplied list replaces
// the built-in one outright rather than extending it.
func (e *Engine) endpoints() []throughput.Endpoint {
	if len(e.opts.Endpoints) > 0 {
		return e.opts.Endpoints
	}
	return throughput.BuiltinEndpoints()
}

// runLatency times the handshake to a throughput endpoint.
//
// This figure is distinct from the per-layer ones: the layers describe segments
// of the path, while this describes the host the throughput figures came from,
// so a slow transfer can be read against the distance it travelled.
//
// It runs before the transfer, never after. A link still draining a throughput
// test would report a latency that describes the test rather than the link.
func (e *Engine) runLatency(ctx context.Context, report *Report) {
	// Both directions skipped means the endpoints are not being touched at all,
	// and timing one would be traffic the user declined.
	if e.opts.SkipDownload && e.opts.SkipUpload {
		return
	}
	target, ports, ok := probeTarget(e.endpoints())
	if !ok {
		return
	}

	report.Latency = latency.Measure(ctx, target, latency.Options{
		Method:  e.opts.LatencyMethod,
		Count:   LatencyProbes,
		Timeout: e.opts.Timeout,
		Ports:   ports,
	})
	if report.Latency.Err != nil {
		report.Errors = append(report.Errors, "latency: "+report.Latency.Err.Error())
	}
}

// probeTarget picks the host to time, and the port to reach it on, from the
// first endpoint that names one.
func probeTarget(endpoints []throughput.Endpoint) (string, []int, bool) {
	for _, endpoint := range endpoints {
		raw := endpoint.DownloadURL
		if raw == "" {
			raw = endpoint.UploadURL
		}
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		// The scheme's default, unless the URL names a port of its own.
		port := 443
		if parsed.Scheme == "http" {
			port = 80
		}
		if explicit := parsed.Port(); explicit != "" {
			if n, err := strconv.Atoi(explicit); err == nil {
				port = n
			}
		}
		return parsed.Hostname(), []int{port}, true
	}
	return "", nil, false
}

func (e *Engine) measureThroughput(ctx context.Context, direction throughput.Direction) throughput.Result {
	endpoints := e.endpoints()
	return retry(ctx, e.opts.Retries, func() (throughput.Result, bool) {
		r := throughput.Measure(ctx, throughput.Options{
			Direction: direction,
			Endpoints: endpoints,
			Samples:   e.opts.Samples,
			Streams:   e.opts.Streams,
			Timeout:   e.opts.Timeout,
		})
		// An endpoint list that is empty will still be empty on the second
		// attempt, so retrying only spends the backoff. Every other failure is
		// the sort a retry exists for.
		permanent := errors.Is(r.Err, throughput.ErrNoEndpoints)
		return r, r.OK || permanent
	})
}

// retry runs fn until it reports success or the attempts run out, doubling the
// delay between attempts.
func retry[T any](ctx context.Context, attempts int, fn func() (T, bool)) T {
	if attempts < 1 {
		attempts = 1
	}
	var result T
	delay := retryBackoff
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return result
			case <-time.After(delay):
			}
			delay *= 2
		}
		var ok bool
		if result, ok = fn(); ok {
			return result
		}
	}
	return result
}

// anySucceeded reports whether the pass learned anything at all.
func anySucceeded(r Report) bool {
	if r.DNS.OK || r.Download.OK || r.Upload.OK || r.Latency.OK {
		return true
	}
	for _, l := range r.Layers {
		if l.Result.OK {
			return true
		}
	}
	return false
}

// hostInfo describes where the run took place. A hostname that cannot be read
// is left empty rather than guessed; it is a label, not a measurement.
func hostInfo() Host {
	name, err := os.Hostname()
	if err != nil {
		name = ""
	}
	return Host{Name: name, OS: runtime.GOOS, Arch: runtime.GOARCH}
}
