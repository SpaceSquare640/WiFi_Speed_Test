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

// ErrNotImplemented reports that throughput measurement is not built yet.
//
// It is returned rather than panicking so that a run which supplies an endpoint
// degrades to a partial report — the layered diagnostics still hold — instead of
// killing the process.
var ErrNotImplemented = errors.New("throughput: measurement is not implemented yet")

// Measure runs the configured direction and returns the aggregate.
func Measure(ctx context.Context, o Options) Result {
	res := Result{Direction: o.Direction}
	if len(o.Endpoints) == 0 {
		res.Err = ErrNoEndpoints
		return res
	}
	res.Err = ErrNotImplemented
	return res
}
