package dns

import (
	"context"
	"testing"
	"time"
)

func TestMeasureDefaults(t *testing.T) {
	res := Measure(context.Background(), Options{Host: "localhost", Timeout: 2 * time.Second})
	if res.Host != "localhost" {
		t.Errorf("Host = %q", res.Host)
	}
	if !res.OK {
		t.Fatalf("resolving localhost failed: %v", res.Err)
	}
	if res.ResolveMS < 0 {
		t.Errorf("ResolveMS = %v, want >= 0", res.ResolveMS)
	}
}

// An address does not need resolving, so timing one would describe nothing.
func TestAddressIsRejected(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		res := Measure(context.Background(), Options{Host: host})
		if res.OK || res.Err == nil {
			t.Errorf("Measure(%q) = %+v; want an error", host, res)
		}
	}
}

func TestUnresolvableName(t *testing.T) {
	res := Measure(context.Background(), Options{Host: "no-such-host.invalid", Timeout: 3 * time.Second})
	if res.OK {
		t.Error("OK = true for an unresolvable name")
	}
	if res.Err == nil {
		t.Error("Err = nil for an unresolvable name")
	}
	// The elapsed time is still reported: how long a failure took to surface is
	// itself a symptom, distinguishing a fast refusal from a stalled resolver.
	if res.ResolveMS < 0 {
		t.Errorf("ResolveMS = %v, want >= 0", res.ResolveMS)
	}
}

func TestEmptyHostUsesDefault(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // do not touch the network; only the defaulting is under test

	res := Measure(ctx, Options{})
	if res.Host != DefaultHost {
		t.Errorf("Host = %q, want %q", res.Host, DefaultHost)
	}
}
