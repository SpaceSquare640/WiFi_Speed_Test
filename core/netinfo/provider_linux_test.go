//go:build linux

package netinfo

import (
	"errors"
	"net/netip"
	"syscall"
	"testing"
)

// androidNetworkTable is a plausible id for one of the per-network routing
// tables netd creates. The exact number does not matter; what matters is that
// it is neither main nor local, because that is the case the old code dropped.
const androidNetworkTable = 1002

func addr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return a
}

func TestSelectGateway(t *testing.T) {
	tests := []struct {
		name   string
		routes func(*testing.T) []routeCandidate
		want   string
	}{
		{
			// The regression this file exists for. Android keeps no default
			// route in main, so filtering to main found nothing at all and the
			// gateway layer vanished on every Android host.
			name: "default route only in a per-network table",
			routes: func(t *testing.T) []routeCandidate {
				return []routeCandidate{
					{addr: addr(t, "10.0.2.2"), table: androidNetworkTable},
				}
			},
			want: "10.0.2.2",
		},
		{
			name: "main table wins over another table",
			routes: func(t *testing.T) []routeCandidate {
				return []routeCandidate{
					{addr: addr(t, "10.0.2.2"), table: androidNetworkTable},
					{addr: addr(t, "192.168.1.1"), table: syscall.RT_TABLE_MAIN},
				}
			},
			want: "192.168.1.1",
		},
		{
			name: "main table wins even on a worse metric",
			routes: func(t *testing.T) []routeCandidate {
				return []routeCandidate{
					{addr: addr(t, "10.0.2.2"), table: androidNetworkTable, priority: 0},
					{addr: addr(t, "192.168.1.1"), table: syscall.RT_TABLE_MAIN, priority: 600},
				}
			},
			want: "192.168.1.1",
		},
		{
			name: "lowest metric wins within one table",
			routes: func(t *testing.T) []routeCandidate {
				return []routeCandidate{
					{addr: addr(t, "192.168.1.254"), table: syscall.RT_TABLE_MAIN, priority: 600},
					{addr: addr(t, "192.168.1.1"), table: syscall.RT_TABLE_MAIN, priority: 100},
				}
			},
			want: "192.168.1.1",
		},
		{
			// IPv4 is preferred ahead of the table, so a v6 route in main does
			// not displace a v4 route from a per-network table.
			name: "IPv4 is preferred over IPv6",
			routes: func(t *testing.T) []routeCandidate {
				return []routeCandidate{
					{addr: addr(t, "fe80::2"), table: syscall.RT_TABLE_MAIN},
					{addr: addr(t, "10.0.2.2"), table: androidNetworkTable},
				}
			},
			want: "10.0.2.2",
		},
		{
			name: "local table is never a way out",
			routes: func(t *testing.T) []routeCandidate {
				return []routeCandidate{
					{addr: addr(t, "127.0.0.1"), table: syscall.RT_TABLE_LOCAL},
					{addr: addr(t, "10.0.2.2"), table: androidNetworkTable},
				}
			},
			want: "10.0.2.2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectGateway(tc.routes(t))
			if err != nil {
				t.Fatalf("selectGateway: unexpected error %v", err)
			}
			if got != tc.want {
				t.Errorf("selectGateway = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSelectGatewayWithNothingUsable(t *testing.T) {
	cases := map[string][]routeCandidate{
		"no routes at all": nil,
		"only local": {
			{addr: netip.MustParseAddr("127.0.0.1"), table: syscall.RT_TABLE_LOCAL},
		},
	}
	for name, routes := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := selectGateway(routes); !errors.Is(err, ErrNotAvailable) {
				t.Errorf("selectGateway error = %v, want ErrNotAvailable", err)
			}
		})
	}
}
