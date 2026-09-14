package layers

import (
	"testing"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/netinfo"
)

func TestResolveAutoDetect(t *testing.T) {
	p := netinfo.Static{Gateway: "192.168.1.1", Resolvers: []string{"203.0.113.53", "203.0.113.54"}}

	set, err := ResolveWith(p, Overrides{})
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	if len(set.Layers) != 3 {
		t.Fatalf("got %d layers, want 3: %+v", len(set.Layers), set.Layers)
	}
	if set.ResolverIsPublic {
		t.Error("a private ISP resolver must not be reported as public")
	}

	want := map[Kind]string{
		KindGateway:        "192.168.1.1",
		KindRegionalEgress: "203.0.113.53", // the first resolver, not the second
		KindInternational:  DefaultInternational,
	}
	for kind, target := range want {
		l, ok := set.Get(kind)
		if !ok {
			t.Errorf("%s: missing", kind)
			continue
		}
		if l.Target != target {
			t.Errorf("%s: got %q, want %q", kind, l.Target, target)
		}
		if l.UserDefined {
			t.Errorf("%s: auto-detected target marked UserDefined", kind)
		}
	}
}

func TestResolveOverridesWin(t *testing.T) {
	p := netinfo.Static{Gateway: "192.168.1.1", Resolvers: []string{"203.0.113.53"}}
	o := Overrides{Gateway: "10.0.0.1", RegionalEgress: "198.51.100.1", International: "example.invalid"}

	set, err := ResolveWith(p, o)
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	for kind, target := range map[Kind]string{
		KindGateway:        "10.0.0.1",
		KindRegionalEgress: "198.51.100.1",
		KindInternational:  "example.invalid",
	} {
		l, ok := set.Get(kind)
		if !ok || l.Target != target {
			t.Errorf("%s: got %q (present=%v), want %q", kind, l.Target, ok, target)
		}
		if !l.UserDefined {
			t.Errorf("%s: override not marked UserDefined", kind)
		}
	}
}

// A host pointed at a public resolver has no ISP-level layer left to measure.
// The set must say so, or two identical rows read as corroboration.
func TestResolverIsPublic(t *testing.T) {
	cases := []struct {
		name      string
		resolvers []string
		want      bool
	}{
		{"ISP resolver", []string{"203.0.113.53"}, false},
		{"known public resolver", []string{"8.8.8.8"}, true},
		{"same as international layer", []string{DefaultInternational}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set, err := ResolveWith(netinfo.Static{Gateway: "192.168.1.1", Resolvers: c.resolvers}, Overrides{})
			if err != nil {
				t.Fatalf("ResolveWith: %v", err)
			}
			if set.ResolverIsPublic != c.want {
				t.Errorf("ResolverIsPublic = %v, want %v", set.ResolverIsPublic, c.want)
			}
		})
	}
}

// A host that can answer nothing still yields the international layer, so the
// run can establish whether the internet is reachable at all.
func TestResolveUnknownHostFacts(t *testing.T) {
	set, err := ResolveWith(netinfo.Static{}, Overrides{})
	if err != nil {
		t.Fatalf("ResolveWith: %v", err)
	}
	if len(set.Layers) != 1 {
		t.Fatalf("got %d layers, want 1: %+v", len(set.Layers), set.Layers)
	}
	if _, ok := set.Get(KindInternational); !ok {
		t.Error("international layer missing")
	}
}

func TestNormaliseRejectsUselessTargets(t *testing.T) {
	// Loopback would measure the host talking to itself and report perfect
	// health for a network that is down.
	for _, target := range []string{"", "0.0.0.0", "127.0.0.1", "::1", "::"} {
		if got, ok := normalise(target); ok {
			t.Errorf("normalise(%q) = %q, true; want rejected", target, got)
		}
	}
	for target, want := range map[string]string{
		"192.168.1.1":    "192.168.1.1",
		"fe80::1%eth0":   "fe80::1",
		"::ffff:8.8.8.8": "8.8.8.8",
		"dns.example":    "dns.example",
	} {
		got, ok := normalise(target)
		if !ok || got != want {
			t.Errorf("normalise(%q) = %q, %v; want %q, true", target, got, ok, want)
		}
	}
}

func TestGatewaySkippedWhenLoopback(t *testing.T) {
	set, _ := ResolveWith(netinfo.Static{Gateway: "127.0.0.1", Resolvers: []string{"203.0.113.53"}}, Overrides{})
	if _, ok := set.Get(KindGateway); ok {
		t.Error("loopback gateway must be omitted, not probed")
	}
}

// A router and a resolver answer on different ports. Probing either on the
// other's port reports a dead layer for a network that is working.
func TestProbePortsDifferByKind(t *testing.T) {
	gateway := Layer{Kind: KindGateway}.ProbePorts()
	if len(gateway) == 0 || gateway[0] != 80 {
		t.Errorf("gateway ports = %v, want the router's web port first", gateway)
	}
	resolver := Layer{Kind: KindRegionalEgress}.ProbePorts()
	if len(resolver) == 0 || resolver[0] != 53 {
		t.Errorf("resolver ports = %v, want the DNS port first", resolver)
	}
	if (Layer{Kind: KindInternational}).ProbePorts()[0] != 53 {
		t.Error("international layer must be probed as a resolver")
	}
}
