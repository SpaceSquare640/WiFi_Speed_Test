package engine

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/grade"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/latency"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

func TestNewFillsDefaults(t *testing.T) {
	e := New(Options{})
	if e.opts.Samples != DefaultSamples || e.opts.Timeout != DefaultTimeout {
		t.Errorf("sampling defaults not applied: %+v", e.opts)
	}
	if e.opts.LatencyMethod != latency.MethodTCP {
		t.Errorf("LatencyMethod = %q, want the privilege-free default", e.opts.LatencyMethod)
	}
	if e.opts.DNSProbeHost == "" {
		t.Error("DNSProbeHost left empty")
	}
}

// A pass that measured nothing is meaningless and must say so, rather than
// returning an empty report that reads like a healthy network.
func TestRunWithoutNetworkReportsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	e := New(Options{SkipLayers: true, DNSProbeHost: "no-such-host.invalid", Retries: 1})
	report, err := e.Run(ctx)
	if !errors.Is(err, ErrNoNetwork) {
		t.Errorf("err = %v, want ErrNoNetwork", err)
	}
	if report.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", report.SchemaVersion, SchemaVersion)
	}
	// Even a failed pass is a report: the caller renders it.
	if report.Timestamp.IsZero() {
		t.Error("Timestamp not set")
	}
	if report.Host.OS != runtime.GOOS || report.Host.Arch != runtime.GOARCH {
		t.Errorf("Host = %+v, want this platform", report.Host)
	}
}

// A partial pass is still worth returning: the layered diagnostics hold even
// when throughput cannot be measured at all.
func TestPartialPassIsNotAnError(t *testing.T) {
	e := New(Options{SkipLayers: true, DNSProbeHost: "localhost", Retries: 1, Timeout: 3 * time.Second})
	report, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !report.DNS.OK {
		t.Fatalf("DNS probe failed: %v", report.DNS.Err)
	}
	// No endpoints are configured, so throughput must report that rather than
	// silently claiming zero.
	if !errors.Is(report.Download.Err, throughput.ErrNoEndpoints) {
		t.Errorf("Download.Err = %v, want ErrNoEndpoints", report.Download.Err)
	}
	if len(report.Errors) == 0 {
		t.Error("a failed component must be recorded in Errors")
	}
	// Reporting "slow" for a rate that was never measured would be a claim
	// without evidence.
	if report.Grade != grade.GradeUnknown {
		t.Errorf("Grade = %q, want %q for an unmeasured rate", report.Grade, grade.GradeUnknown)
	}
}

func TestRetryStopsOnSuccess(t *testing.T) {
	calls := 0
	got := retry(context.Background(), 5, func() (int, bool) {
		calls++
		return calls, calls == 2
	})
	if calls != 2 {
		t.Errorf("fn called %d times, want 2", calls)
	}
	if got != 2 {
		t.Errorf("got %d, want the successful result", got)
	}
}

func TestRetryExhaustsAttemptsAndKeepsLastResult(t *testing.T) {
	calls := 0
	start := time.Now()
	got := retry(context.Background(), 3, func() (int, bool) {
		calls++
		return calls, false
	})
	if calls != 3 {
		t.Errorf("fn called %d times, want 3", calls)
	}
	if got != 3 {
		t.Errorf("got %d, want the last attempt's result", got)
	}
	// Backoff doubles: 500ms then 1s between the three attempts.
	if elapsed := time.Since(start); elapsed < 1400*time.Millisecond {
		t.Errorf("elapsed %v, want the backoff to have been honoured", elapsed)
	}
}

func TestRetryHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	retry(ctx, 5, func() (int, bool) {
		calls++
		return 0, false
	})
	if calls != 1 {
		t.Errorf("fn called %d times after cancellation, want 1", calls)
	}
}

func TestAnySucceeded(t *testing.T) {
	if anySucceeded(Report{}) {
		t.Error("an empty report must not count as success")
	}
	// One responding layer is enough: a run that cannot reach the internet has
	// still established that the gateway answers.
	r := Report{Layers: []LayerResult{{Result: latency.Result{OK: true}}}}
	if !anySucceeded(r) {
		t.Error("a responding layer must count as success")
	}
}
