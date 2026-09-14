// Package latency measures round-trip time, jitter and packet loss against a
// single target.
//
// The default probe is a TCP handshake rather than ICMP: ICMP requires elevated
// privileges on Windows and on Android, and silently degrading to a failure
// there would make the tool useless for most users. ICMP remains available as an
// opt-in for hosts that can grant it.
package latency

import (
	"context"
	"errors"
	"math"
	"net"
	"strconv"
	"time"
)

// Method selects the probe mechanism.
type Method string

const (
	MethodTCP  Method = "tcp"
	MethodICMP Method = "icmp"
)

// Defaults applied to a zero-valued Options.
const (
	DefaultCount   = 5
	DefaultTimeout = 2 * time.Second

	// DefaultPort is the HTTPS port, which is the one most widely answered:
	// routers, resolvers and public hosts alike. What matters is that something
	// replies, not what it replies with.
	DefaultPort = 443

	// probeGap separates consecutive probes. Back-to-back handshakes measure the
	// host's own scheduling as much as the network, and can look like a flood to
	// the target.
	probeGap = 100 * time.Millisecond
)

// Options configures a measurement.
type Options struct {
	Method  Method
	Count   int
	Timeout time.Duration

	// Port is the destination port for MethodTCP.
	Port int

	// Ports are candidate destination ports, tried in order until one answers.
	//
	// A single port is not enough in practice: a home router may answer on 80,
	// on 443, on 53, or on none of them depending on the model, while a resolver
	// answers only on 53. Probing a router on the wrong port reports a dead
	// first layer for a network that is working, which is the worst failure this
	// tool can produce. When empty, Port alone is used.
	Ports []int
}

// withDefaults fills unset fields, so that a zero Options is usable.
func (o Options) withDefaults() Options {
	if o.Method == "" {
		o.Method = MethodTCP
	}
	if o.Count <= 0 {
		o.Count = DefaultCount
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	if o.Port <= 0 {
		o.Port = DefaultPort
	}
	if len(o.Ports) == 0 {
		o.Ports = []int{o.Port}
	}
	return o
}

// Result is the outcome of probing one target.
type Result struct {
	Target string
	OK     bool

	// LatencyMS is the mean round-trip time over the successful samples.
	LatencyMS float64

	// JitterMS is the standard deviation of the successful samples. It is zero
	// when fewer than two samples succeeded.
	JitterMS float64

	// LossPct is the share of probes that received no reply, 0 to 100.
	LossPct float64

	// Samples holds every successful round-trip time, in milliseconds.
	Samples []float64

	// ProbePort is the port that answered, for MethodTCP. It belongs in the
	// report: "no reply on 443" and "no reply anywhere" are different findings,
	// and the user can only tell them apart if the port is stated.
	ProbePort int

	Err error
}

// outcome classifies what a single probe learned.
type outcome int

const (
	// replied means a round trip completed, whether the peer accepted the
	// connection or rejected it.
	replied outcome = iota

	// lost means nothing came back: a timeout, or a path with no route.
	lost

	// fatal means the probe could not be sent at all, which is a property of
	// the target rather than of the network between here and it.
	fatal
)

// Measure probes target and returns the aggregate result. A target that never
// replies yields Result{OK: false, LossPct: 100}, not an error: an unreachable
// layer is a diagnostic finding, not a failure of the tool.
func Measure(ctx context.Context, target string, o Options) Result {
	o = o.withDefaults()
	res := Result{Target: target}

	if target == "" {
		res.Err = errors.New("latency: no target")
		res.LossPct = 100
		return res
	}

	probe := tcpProbe
	if o.Method == MethodICMP {
		probe = icmpProbe
	} else {
		// Settle on a port that answers before measuring. Mixing ports within
		// one run would average unlike paths into a single meaningless figure.
		port, err := selectPort(ctx, target, o)
		if err != nil {
			res.Err = err
			res.LossPct = 100
			return res
		}
		o.Port = port
		res.ProbePort = port
	}

	attempted := 0
	for i := 0; i < o.Count; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				// Cancellation ends the run; probes never sent are not losses.
				return summarise(res, attempted)
			case <-time.After(probeGap):
			}
		}
		if ctx.Err() != nil {
			return summarise(res, attempted)
		}

		attempted++
		rtt, state, err := probe(ctx, target, o)
		switch state {
		case replied:
			res.Samples = append(res.Samples, float64(rtt)/float64(time.Millisecond))
		case fatal:
			// A target that cannot be probed at all is reported once and the
			// run stops: repeating it would only restate the same cause.
			if res.Err == nil {
				res.Err = err
			}
			attempted--
			return summarise(res, attempted)
		}
	}

	return summarise(res, attempted)
}

// summarise derives the aggregate figures from the collected samples.
func summarise(res Result, attempted int) Result {
	if attempted == 0 {
		if res.Err == nil {
			res.LossPct = 100
		}
		return res
	}

	lost := attempted - len(res.Samples)
	res.LossPct = float64(lost) / float64(attempted) * 100

	if len(res.Samples) == 0 {
		return res
	}
	res.OK = true

	var sum float64
	for _, s := range res.Samples {
		sum += s
	}
	res.LatencyMS = sum / float64(len(res.Samples))

	// Jitter is the spread of the round trips. One sample has no spread, and
	// reporting zero would be indistinguishable from a perfectly stable link.
	if len(res.Samples) > 1 {
		var variance float64
		for _, s := range res.Samples {
			d := s - res.LatencyMS
			variance += d * d
		}
		res.JitterMS = math.Sqrt(variance / float64(len(res.Samples)))
	}
	return res
}

// selectPort finds the first candidate port the target answers on.
//
// When none answers, the last candidate is used anyway: the run then reports
// 100% loss against a named port, which is an honest finding, rather than
// refusing to measure at all.
func selectPort(ctx context.Context, target string, o Options) (int, error) {
	if len(o.Ports) == 1 {
		return o.Ports[0], nil
	}
	for _, port := range o.Ports {
		if ctx.Err() != nil {
			return port, nil
		}
		probeOpts := o
		probeOpts.Port = port
		_, state, err := tcpProbe(ctx, target, probeOpts)
		switch state {
		case replied:
			return port, nil
		case fatal:
			return 0, err
		}
	}
	return o.Ports[len(o.Ports)-1], nil
}

// tcpProbe times a TCP handshake against the target.
//
// A refused connection counts as a reply, not a loss. The port being closed is
// irrelevant: the RST proves the packet reached the host and came back, which is
// exactly what the measurement is asking. Only silence is loss.
func tcpProbe(ctx context.Context, target string, o Options) (time.Duration, outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	addr := net.JoinHostPort(target, strconv.Itoa(o.Port))
	var d net.Dialer

	clock := startTimer()
	conn, err := d.DialContext(ctx, "tcp", addr)
	elapsed := clock.elapsed()

	if err == nil {
		conn.Close()
		return elapsed, replied, nil
	}
	return elapsed, classify(err), err
}

// classify decides what a dial error means for the measurement.
func classify(err error) outcome {
	// A name that does not resolve is not a lost packet; it is a target that
	// does not exist, and the run should say so rather than report 100% loss.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fatal
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return lost
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return lost
	}
	// A rejection is a round trip: something answered.
	if isRefused(err) {
		return replied
	}
	// Anything else — no route, network down, address unusable — means the
	// packet did not come back, and treating it as a reply would invent a
	// round-trip time out of a local failure.
	return lost
}
