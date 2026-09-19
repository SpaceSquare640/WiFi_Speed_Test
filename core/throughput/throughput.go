// Package throughput measures download and upload rates over plain HTTP against
// public endpoints.
//
// The protocol is deliberately ordinary. An opaque vendor protocol would make it
// impossible to tell users honestly where their traffic goes, and would tie the
// project to terms it cannot control.
package throughput

import (
	"context"
	"errors"
	"time"
)

// Direction selects which way the data flows.
type Direction string

const (
	DirectionDownload Direction = "download"
	DirectionUpload   Direction = "upload"
)

// Endpoint is one measurement target.
//
// The built-in list ships empty until each candidate has been cleared for terms
// of service, rate limits, upload support, stability and geographic coverage.
// Filling it with unverified endpoints would merely relocate the licensing risk
// this design exists to avoid.
type Endpoint struct {
	Name        string
	DownloadURL string
	UploadURL   string
	Provider    string
}

// BuiltinEndpoints returns the endpoints shipped with this build.
func BuiltinEndpoints() []Endpoint { return nil }

// Options configures a measurement run.
type Options struct {
	Direction Direction

	// Endpoints are measured in order until Samples results are collected.
	// A user-supplied list replaces the built-in one outright; mixing the two
	// would average across unlike conditions and distort the result.
	Endpoints []Endpoint

	// Samples is how many endpoints contribute to the mean.
	Samples int

	Timeout time.Duration

	// Streams is the number of concurrent connections per endpoint.
	Streams int
}

// Sample is one endpoint's contribution.
type Sample struct {
	Endpoint string
	Mbps     float64
	Err      error
}

// Result aggregates the samples for one direction.
type Result struct {
	Direction Direction
	OK        bool

	// Mbps is the mean across successful samples.
	Mbps float64

	Samples []Sample
	Err     error
}

// ErrNoEndpoints reports that there was nothing to measure against. The builtin
// list ships empty until each candidate has been cleared, so this is the normal
// state of a fresh install rather than a fault.
var ErrNoEndpoints = errors.New("throughput: no endpoint configured (supply one with --endpoint)")

// ErrDirectionUnsupported reports that an endpoint carries no URL for the
// direction being measured, which is a configuration fact rather than a
// failure: an endpoint may serve downloads and refuse uploads.
var ErrDirectionUnsupported = errors.New("throughput: endpoint has no URL for this direction")

// ErrAllEndpointsFailed reports that every endpoint was tried and none of them
// yielded a measurement. The per-endpoint reasons are in Result.Samples; this
// error only says that nothing survived to be averaged.
var ErrAllEndpointsFailed = errors.New("throughput: every endpoint failed")

// Defaults applied to a zero-valued Options.
const (
	DefaultSamples = 3
	DefaultTimeout = 10 * time.Second

	// DefaultStreams is one connection.
	//
	// A single stream is what the figure claims to be — the rate one TCP
	// connection achieved — and needs no caveat to explain. It does understate
	// a long, high-latency path, where one connection's window bounds it below
	// what the link can carry; --streams raises it for anyone measuring such a
	// path, and the result then describes several connections rather than one.
	DefaultStreams = 1
)

// Measure runs the configured direction and returns the aggregate.
//
// Endpoints are measured one at a time, never together. Concurrent transfers to
// different endpoints would contend for the same link and each would report a
// share of it, so the mean would describe the contention rather than the
// connection.
func Measure(ctx context.Context, o Options) Result {
	res := Result{Direction: o.Direction}
	if len(o.Endpoints) == 0 {
		res.Err = ErrNoEndpoints
		return res
	}
	if o.Samples <= 0 {
		o.Samples = DefaultSamples
	}
	if o.Streams <= 0 {
		o.Streams = DefaultStreams
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}

	var sum float64
	var measured int

	for _, endpoint := range o.Endpoints {
		if measured >= o.Samples || ctx.Err() != nil {
			break
		}

		url := endpoint.DownloadURL
		if o.Direction == DirectionUpload {
			url = endpoint.UploadURL
		}
		name := endpoint.Name
		if name == "" {
			name = url
		}
		if url == "" {
			res.Samples = append(res.Samples, Sample{Endpoint: name, Err: ErrDirectionUnsupported})
			continue
		}

		transferred, err := run(ctx, o.Direction, url, o.Streams, o.Timeout)
		if err != nil {
			// One endpoint failing is a reason to try the next, not to abandon
			// the direction; the reason is kept so the user can see it.
			res.Samples = append(res.Samples, Sample{Endpoint: name, Err: err})
			continue
		}

		mbps := rate(transferred)
		res.Samples = append(res.Samples, Sample{Endpoint: name, Mbps: mbps})
		sum += mbps
		measured++
	}

	if measured == 0 {
		res.Err = ErrAllEndpointsFailed
		return res
	}
	res.OK = true
	res.Mbps = sum / float64(measured)
	return res
}

// rate converts a transfer to megabits per second.
//
// Decimal mega, not mebi: link rates are quoted in powers of ten everywhere
// they are sold, and reporting 1024-based numbers under the same name would
// understate every figure by about five per cent against what the user was
// promised.
func rate(t transferResult) float64 {
	seconds := t.elapsed.Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(t.bytes) * 8 / seconds / 1e6
}
