// Package dns measures name resolution time, which is reported separately from
// latency because slow resolution and slow transport have different causes and
// different fixes.
package dns

import (
	"context"
	"errors"
	"net"
	"runtime"
	"time"
)

// DefaultHost is the name resolved when the caller names none.
//
// It is a domain reserved by the IANA for documentation, so resolving it asks
// nothing of anybody's production service. Like every other target in this tool
// it can be overridden.
const DefaultHost = "example.com"

// DefaultTimeout bounds one lookup.
const DefaultTimeout = 5 * time.Second

// ErrUnsupported reports that resolution time cannot be measured on this
// platform.
//
// Android is the case in hand. It keeps no /etc/resolv.conf, and its real
// resolver lives behind netd, reachable only through bionic — which a binary
// built without cgo cannot call. Go's own resolver therefore falls back to
// localhost, where nothing listens, and every lookup fails with a connection
// refused that describes the fallback rather than the network. Reporting that
// as a DNS fault would be worse than reporting nothing, so the measurement is
// declined outright and said to be unsupported.
var ErrUnsupported = errors.New("dns: resolution time cannot be measured on this platform")

// Options configures a resolution measurement.
type Options struct {
	Host    string
	Timeout time.Duration
}

// Result is the outcome of one resolution.
type Result struct {
	Host string
	OK   bool

	// ResolveMS is the elapsed time of the lookup, in milliseconds.
	ResolveMS float64

	Err error
}

// Measure resolves the configured host and reports how long it took.
//
// The host resolver is used as configured, without bypassing the system's own
// cache. A cached answer is a fast answer, and the figure is meant to describe
// what an application on this machine would actually experience.
func Measure(ctx context.Context, o Options) Result {
	if o.Host == "" {
		o.Host = DefaultHost
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	res := Result{Host: o.Host}

	if runtime.GOOS == "android" {
		res.Err = ErrUnsupported
		return res
	}

	// An address needs no resolving; reporting a near-zero time for it would
	// describe nothing.
	if net.ParseIP(o.Host) != nil {
		res.Err = errors.New("dns: probe host is an address, not a name")
		return res
	}

	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	clock := startTimer()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, o.Host)
	elapsed := clock.elapsed()

	res.ResolveMS = float64(elapsed) / float64(time.Millisecond)
	if err != nil {
		res.Err = err
		return res
	}
	if len(addrs) == 0 {
		res.Err = &net.DNSError{Err: "no addresses returned", Name: o.Host}
		return res
	}
	res.OK = true
	return res
}
