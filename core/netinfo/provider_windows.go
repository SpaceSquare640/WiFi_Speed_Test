//go:build windows

package netinfo

import (
	"net/netip"
	"sort"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Adapter states and types, declared here rather than taken from the x/sys
// constants so that the meaning is visible at the point of use.
const (
	ifOperStatusUp         = 1
	ifTypeSoftwareLoopback = 24
)

type windowsProvider struct{}

func platformProvider() Provider { return windowsProvider{} }

// candidate is one address with the interface metric that ranks it. Windows
// exposes several adapters at once — a wired link, Wi-Fi, and any number of VPN
// or virtual adapters — and the metric is how the system itself decides between
// them. Ranking by it is what makes this agree with the route the traffic will
// actually take.
type candidate struct {
	addr   netip.Addr
	metric uint32
}

func (windowsProvider) DefaultGateway() (string, error) {
	adapters, err := adapterList()
	if err != nil {
		return "", err
	}

	var found []candidate
	for _, a := range adapters {
		if !usableAdapter(a) {
			continue
		}
		for gw := a.FirstGatewayAddress; gw != nil; gw = gw.Next {
			addr, ok := sockaddrToAddr(gw.Address)
			if !ok || !addr.IsValid() || addr.IsUnspecified() {
				continue
			}
			metric := a.Ipv4Metric
			if addr.Is6() {
				metric = a.Ipv6Metric
			}
			found = append(found, candidate{addr: addr, metric: metric})
		}
	}
	if len(found) == 0 {
		return "", ErrNotAvailable
	}

	// IPv4 first: the diagnostic layers probe a single address, and an IPv4
	// gateway is reachable on every host this tool targets.
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].addr.Is4() != found[j].addr.Is4() {
			return found[i].addr.Is4()
		}
		return found[i].metric < found[j].metric
	})
	return found[0].addr.Unmap().String(), nil
}

func (windowsProvider) SystemResolvers() ([]string, error) {
	adapters, err := adapterList()
	if err != nil {
		return nil, err
	}

	var found []candidate
	for _, a := range adapters {
		if !usableAdapter(a) {
			continue
		}
		for dns := a.FirstDnsServerAddress; dns != nil; dns = dns.Next {
			addr, ok := sockaddrToAddr(dns.Address)
			if !ok || !addr.IsValid() || addr.IsUnspecified() {
				continue
			}
			metric := a.Ipv4Metric
			if addr.Is6() {
				metric = a.Ipv6Metric
			}
			found = append(found, candidate{addr: addr, metric: metric})
		}
	}

	sort.SliceStable(found, func(i, j int) bool { return found[i].metric < found[j].metric })

	seen := make(map[netip.Addr]bool, len(found))
	out := make([]string, 0, len(found))
	for _, c := range found {
		a := c.addr.Unmap()
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a.String())
	}
	if len(out) == 0 {
		return nil, ErrNotAvailable
	}
	return out, nil
}

// usableAdapter rejects adapters that cannot carry the probes: anything down,
// and the loopback, whose gateway would be a target that never leaves the host.
func usableAdapter(a *windows.IpAdapterAddresses) bool {
	return a.OperStatus == ifOperStatusUp && a.IfType != ifTypeSoftwareLoopback
}

// adapterList returns the host's adapters as a slice.
//
// GetAdaptersAddresses reports the required buffer size by failing, so the call
// is retried on overflow. The bound exists because the adapter set can change
// between calls and an unbounded loop would hang rather than fail.
func adapterList() ([]*windows.IpAdapterAddresses, error) {
	// INCLUDE_GATEWAYS is required: without it FirstGatewayAddress is left nil
	// on every adapter and the gateway silently disappears.
	const flags = windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST |
		windows.GAA_FLAG_INCLUDE_GATEWAYS

	size := uint32(15000)
	var buf []byte
	var ok bool
	for attempt := 0; attempt < 4; attempt++ {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(
			windows.AF_UNSPEC,
			flags,
			0,
			(*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])),
			&size,
		)
		if err == nil {
			ok = true
			break
		}
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		return nil, err
	}
	if !ok {
		return nil, ErrNotAvailable
	}

	var out []*windows.IpAdapterAddresses
	for a := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
		out = append(out, a)
	}
	return out, nil
}

// sockaddrToAddr reads the address out of a raw Windows socket address.
//
// The family tag decides how the rest of the structure is interpreted; there is
// no safe way to read it without checking that first.
func sockaddrToAddr(sa windows.SocketAddress) (netip.Addr, bool) {
	if sa.Sockaddr == nil {
		return netip.Addr{}, false
	}
	switch sa.Sockaddr.Addr.Family {
	case windows.AF_INET:
		v := (*syscall.RawSockaddrInet4)(unsafe.Pointer(sa.Sockaddr))
		return netip.AddrFrom4(v.Addr), true
	case windows.AF_INET6:
		v := (*syscall.RawSockaddrInet6)(unsafe.Pointer(sa.Sockaddr))
		return netip.AddrFrom16(v.Addr), true
	default:
		return netip.Addr{}, false
	}
}
