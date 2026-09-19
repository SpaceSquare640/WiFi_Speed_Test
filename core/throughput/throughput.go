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
	"fmt"
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

	// Streams is how many connections carried the measurement. It travels with
	// the figure because the figure means a different thing at one connection
	// than at four, and a number whose method is not stated invites the reader
	// to assume the wrong one.
	Streams int

	// Mbps is the mean across successful samples.
	Mbps float64

	Samples []Sample
	Err     error
}

// ErrNoEndpoints reports that there was nothing to measure against. The builtin
// list ships empty until each candidate has been cleared, so this is the normal
// state of a fresh install rather than a fault.
var ErrNoEndpoints = errors.New("throughput: no endpoint configured")

// noEndpointFor reports that endpoints were supplied but none of them serves
// this direction — a download-only list asked for an upload, say.
//
// It wraps ErrNoEndpoints because the situation is the same one: there is
// nothing to measure against and no retry will change that. Only the advice
// differs, and the advice is the part the user needs.
func noEndpointFor(d Direction) error {
	flag := "--download-endpoint"
	if d == DirectionUpload {
		flag = "--upload-endpoint"
	}
	return fmt.Errorf("%w for %s; supply one with %s", ErrNoEndpoints, d, flag)
}

// ErrAllEndpointsFailed reports that every endpoint was tried and none of them
// yielded a measurement. The per-endpoint reasons are in Result.Samples; this
// error only says that nothing survived to be averaged.
var ErrAllEndpointsFailed = errors.New("throughput: every endpoint failed")

// Defaults applied to a zero-valued Options.
const (
	DefaultSamples = 3
	DefaultTimeout = 10 * time.Second

	// DefaultStreams is four connections.
	//
	// One would be the more transparent figure and was the original default,
	// until a measurement against a real endpoint showed what it costs: upload
	// over four connections came out 3.6 times higher than over one, and rose
	// almost linearly, which is the signature of a single connection's
	// congestion window rather than of the link. A default that reports a
	// healthy upload as a twentieth of its capacity would send users hunting a
	// fault that is not there, and for a tool whose job is to say what is
	// wrong, inventing a fault is the worst error available.
	//
	// The figure therefore describes four connections, and every renderer says
	// so beside it. --streams 1 restores the stricter reading.
	DefaultStreams = 4
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
		res.Err = fmt.Errorf("%w; supply one with --endpoint", ErrNoEndpoints)
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

	res.Streams = o.Streams

	// An endpoint that serves only the other direction is not a failure worth
	// reporting: a download-only endpoint in the list is a deliberate
	// configuration, and a sample saying so for every one of them would bury
	// the failures that do matter.
	serving := make([]Endpoint, 0, len(o.Endpoints))
	for _, endpoint := range o.Endpoints {
		if directionURL(endpoint, o.Direction) != "" {
			serving = append(serving, endpoint)
		}
	}
	if len(serving) == 0 {
		res.Err = noEndpointFor(o.Direction)
		return res
	}

	var sum float64
	var measured int

	for _, endpoint := range serving {
		if measured >= o.Samples || ctx.Err() != nil {
			break
		}

		url := directionURL(endpoint, o.Direction)
		name := endpoint.Name
		if name == "" {
			name = url
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

// directionURL returns the endpoint's URL for one direction, or empty when it
// does not serve that direction at all.
func directionURL(e Endpoint, d Direction) string {
	if d == DirectionUpload {
		return e.UploadURL
	}
	return e.DownloadURL
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
