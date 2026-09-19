// Package output renders a report for a destination.
package output

import (
	"io"
	"math"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

// Writer renders reports.
type Writer interface {
	// Write emits one report. In continuous mode it is called once per cycle.
	Write(r engine.Report, t history.Trend) error

	// Close flushes anything held back for the end of the run.
	Close() error
}

// Options configures a writer.
type Options struct {
	Out   io.Writer
	Color bool
	Lang  string

	// Watch tells the writer that more reports will follow, which changes the
	// framing: JSON emits one object per line rather than a single document.
	Watch bool
}

// New returns the writer for the requested format.
func New(asJSON bool, o Options) Writer {
	if asJSON {
		return NewJSON(o)
	}
	return NewHuman(o)
}

// round trims a measurement to three decimal places.
//
// The extra digits are not precision, they are noise from floating point
// arithmetic, and emitting them invites readers to believe a sub-microsecond
// figure means something.
func round(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*1000) / 1000
}

// throughputToWire carries the connection count alongside the figure. A
// consumer comparing two runs needs to know whether they were measured the
// same way, and the rate alone does not say.
func throughputToWire(ok bool, mbps float64, streams int, err error, samples []throughput.Sample) wireThroughput {
	out := wireThroughput{OK: ok, Mbps: round(mbps), Streams: streams, Error: errString(err)}
	for _, s := range samples {
		out.Samples = append(out.Samples, wireSample{
			Endpoint: s.Endpoint,
			Mbps:     round(s.Mbps),
			Error:    errString(s.Err),
		})
	}
	return out
}
