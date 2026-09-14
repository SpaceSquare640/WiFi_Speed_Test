package latency

import (
	"context"
	"math"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestMeasureAgainstListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	host, port := splitPort(t, ln.Addr().String())
	res := Measure(context.Background(), host, Options{Count: 3, Port: port, Timeout: time.Second})

	if !res.OK {
		t.Fatalf("OK = false, err = %v", res.Err)
	}
	if len(res.Samples) != 3 {
		t.Errorf("got %d samples, want 3", len(res.Samples))
	}
	if res.LossPct != 0 {
		t.Errorf("LossPct = %v, want 0", res.LossPct)
	}
	if res.LatencyMS <= 0 {
		t.Errorf("LatencyMS = %v, want > 0", res.LatencyMS)
	}
}

// A closed port still proves the host is there and answering. Counting the RST
// as loss would blame the network for a port that was simply not open.
func TestClosedPortCountsAsReply(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, port := splitPort(t, ln.Addr().String())
	ln.Close() // nothing is listening now

	res := Measure(context.Background(), host, Options{Count: 2, Port: port, Timeout: time.Second})
	if !res.OK {
		t.Fatalf("a refused connection must count as a reply; err = %v", res.Err)
	}
	if res.LossPct != 0 {
		t.Errorf("LossPct = %v, want 0", res.LossPct)
	}
}

// A name that does not exist is a broken target, not a lost packet.
func TestUnresolvableTargetIsFatal(t *testing.T) {
	res := Measure(context.Background(), "no-such-host.invalid", Options{Count: 3, Timeout: time.Second})
	if res.OK {
		t.Error("OK = true for an unresolvable target")
	}
	if res.Err == nil {
		t.Error("Err = nil; an unresolvable target must be reported, not silently counted as loss")
	}
	if len(res.Samples) != 0 {
		t.Errorf("got %d samples, want 0", len(res.Samples))
	}
}

func TestEmptyTarget(t *testing.T) {
	res := Measure(context.Background(), "", Options{})
	if res.OK || res.Err == nil || res.LossPct != 100 {
		t.Errorf("got %+v, want not OK with an error and 100%% loss", res)
	}
}

func TestCancellationStopsWithoutCountingLoss(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := Measure(ctx, "192.0.2.1", Options{Count: 5, Timeout: 100 * time.Millisecond})
	if len(res.Samples) != 0 {
		t.Errorf("got %d samples after cancellation", len(res.Samples))
	}
}

func TestSummariseStatistics(t *testing.T) {
	// Four replies out of five attempts: mean 20, population stddev 5.
	res := summarise(Result{Samples: []float64{15, 15, 25, 25}}, 5)

	if !res.OK {
		t.Fatal("OK = false with samples present")
	}
	if res.LatencyMS != 20 {
		t.Errorf("LatencyMS = %v, want 20", res.LatencyMS)
	}
	if math.Abs(res.JitterMS-5) > 1e-9 {
		t.Errorf("JitterMS = %v, want 5", res.JitterMS)
	}
	if math.Abs(res.LossPct-20) > 1e-9 {
		t.Errorf("LossPct = %v, want 20", res.LossPct)
	}
}

// One sample has no spread. Reporting a jitter of zero would be
// indistinguishable from a link measured many times and found perfectly stable.
func TestSingleSampleHasNoJitter(t *testing.T) {
	res := summarise(Result{Samples: []float64{12}}, 1)
	if res.JitterMS != 0 {
		t.Errorf("JitterMS = %v, want 0", res.JitterMS)
	}
	if res.LatencyMS != 12 {
		t.Errorf("LatencyMS = %v, want 12", res.LatencyMS)
	}
}

func TestTotalLoss(t *testing.T) {
	res := summarise(Result{}, 4)
	if res.OK {
		t.Error("OK = true with no samples")
	}
	if res.LossPct != 100 {
		t.Errorf("LossPct = %v, want 100", res.LossPct)
	}
}

func TestOptionDefaults(t *testing.T) {
	o := Options{}.withDefaults()
	if o.Method != MethodTCP || o.Count != DefaultCount || o.Timeout != DefaultTimeout || o.Port != DefaultPort {
		t.Errorf("zero Options did not fill defaults: %+v", o)
	}
}

func splitPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}
