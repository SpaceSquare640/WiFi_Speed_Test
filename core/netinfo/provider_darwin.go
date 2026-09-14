//go:build darwin

package netinfo

import (
	"net/netip"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

type darwinProvider struct{}

func platformProvider() Provider { return darwinProvider{} }

// DefaultGateway reads the routing table through the sysctl routing interface,
// which is how the BSD kernels expose it as structured data.
func (darwinProvider) DefaultGateway() (string, error) {
	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return "", ErrNotAvailable
	}
	msgs, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return "", ErrNotAvailable
	}

	var fallback netip.Addr
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok {
			continue
		}
		const want = unix.RTF_UP | unix.RTF_GATEWAY
		if rm.Flags&want != want {
			continue
		}
		if len(rm.Addrs) <= unix.RTAX_GATEWAY {
			continue
		}
		if !isDefaultDestination(rm.Addrs[unix.RTAX_DST]) {
			continue
		}
		addr, ok := routeAddrToAddr(rm.Addrs[unix.RTAX_GATEWAY])
		if !ok || !addr.IsValid() || addr.IsUnspecified() {
			continue
		}
		// IPv4 wins outright; an IPv6 default is held back in case no IPv4
		// default exists.
		if addr.Is4() {
			return addr.String(), nil
		}
		if !fallback.IsValid() {
			fallback = addr
		}
	}
	if fallback.IsValid() {
		return fallback.Unmap().String(), nil
	}
	return "", ErrNotAvailable
}

func (darwinProvider) SystemResolvers() ([]string, error) { return resolvConfNameservers() }

// isDefaultDestination reports whether a destination is the all-zero address,
// which is what marks a route as the default one.
func isDefaultDestination(a route.Addr) bool {
	addr, ok := routeAddrToAddr(a)
	return ok && addr.IsUnspecified()
}

func routeAddrToAddr(a route.Addr) (netip.Addr, bool) {
	switch v := a.(type) {
	case *route.Inet4Addr:
		return netip.AddrFrom4(v.IP), true
	case *route.Inet6Addr:
		return netip.AddrFrom16(v.IP), true
	default:
		return netip.Addr{}, false
	}
}
