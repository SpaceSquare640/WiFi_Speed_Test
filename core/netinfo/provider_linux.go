//go:build linux

package netinfo

import (
	"encoding/binary"
	"net/netip"
	"runtime"
	"sort"
	"syscall"
	"unsafe"
)

type linuxProvider struct{}

func platformProvider() Provider { return linuxProvider{} }

// DefaultGateway reads the kernel routing table over netlink.
//
// Netlink is used rather than /proc/net/route because Android restricts /proc
// network files from API 29 onward, and the Termux build shares this code.
//
// Every table except the local one is searched. Filtering to RT_TABLE_MAIN
// would be correct on a desktop Linux and wrong on Android, where netd has kept
// the default route in a per-network table selected by fwmark rules since
// Android 5 — main holds only the on-link subnet there, so the gateway would
// never be found. The local table is excluded because it holds this host's own
// addresses and its broadcast entries, never a way off the machine.
func (linuxProvider) DefaultGateway() (string, error) {
	rib, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_UNSPEC)
	if err != nil {
		return "", ErrNotAvailable
	}
	msgs, err := syscall.ParseNetlinkMessage(rib)
	if err != nil {
		return "", ErrNotAvailable
	}

	var routes []routeCandidate

	for _, m := range msgs {
		if m.Header.Type == syscall.NLMSG_DONE {
			break
		}
		if m.Header.Type != syscall.RTM_NEWROUTE {
			continue
		}
		if len(m.Data) < int(unsafe.Sizeof(syscall.RtMsg{})) {
			continue
		}
		rt := (*syscall.RtMsg)(unsafe.Pointer(&m.Data[0]))

		// A default route is the one with no destination prefix. Anything else
		// describes a subnet, not the way out of it.
		if rt.Dst_len != 0 {
			continue
		}
		if rt.Type != syscall.RTN_UNICAST {
			continue
		}

		attrs, err := syscall.ParseNetlinkRouteAttr(&m)
		if err != nil {
			continue
		}
		// RtMsg.Table is a single byte, so a table id above 255 — which is
		// every per-network table Android creates — cannot be expressed there.
		// The kernel reports RT_TABLE_UNSPEC or RT_TABLE_COMPAT in the header
		// and carries the real id in this attribute, so the attribute wins
		// whenever it is present.
		table := uint32(rt.Table)
		var gw netip.Addr
		var priority uint32
		for _, a := range attrs {
			switch a.Attr.Type {
			case syscall.RTA_GATEWAY:
				if addr, ok := netip.AddrFromSlice(a.Value); ok {
					gw = addr
				}
			case syscall.RTA_PRIORITY:
				if len(a.Value) >= 4 {
					priority = binary.NativeEndian.Uint32(a.Value[:4])
				}
			case syscall.RTA_TABLE:
				if len(a.Value) >= 4 {
					table = binary.NativeEndian.Uint32(a.Value[:4])
				}
			}
		}
		// A default route with no next hop is on-link — Android's dummy0
		// placeholder is one — and names no gateway to probe.
		if gw.IsValid() && !gw.IsUnspecified() {
			routes = append(routes, routeCandidate{addr: gw, priority: priority, table: table})
		}
	}
	return selectGateway(routes)
}

// routeCandidate is one default route the kernel offered.
type routeCandidate struct {
	addr     netip.Addr
	priority uint32
	table    uint32
}

// selectGateway picks which default route names the next hop to probe.
//
// IPv4 first, then the main table, then the lowest metric. Ordering the main
// table ahead of the others keeps a desktop Linux answering exactly as it did
// before, since there every default route lives in main; the tie-break only
// decides anything on a host like Android, which puts them elsewhere.
func selectGateway(routes []routeCandidate) (string, error) {
	// The local table holds this host's own addresses and its broadcast
	// entries, never a way off the machine.
	usable := make([]routeCandidate, 0, len(routes))
	for _, r := range routes {
		if r.table == syscall.RT_TABLE_LOCAL {
			continue
		}
		usable = append(usable, r)
	}
	if len(usable) == 0 {
		return "", ErrNotAvailable
	}

	sort.SliceStable(usable, func(i, j int) bool {
		if usable[i].addr.Is4() != usable[j].addr.Is4() {
			return usable[i].addr.Is4()
		}
		iMain := usable[i].table == syscall.RT_TABLE_MAIN
		jMain := usable[j].table == syscall.RT_TABLE_MAIN
		if iMain != jMain {
			return iMain
		}
		return usable[i].priority < usable[j].priority
	})
	return usable[0].addr.Unmap().String(), nil
}

// SystemResolvers reads the host's configured resolvers.
//
// Android is reported as unsupported rather than merely unavailable. It keeps no
// userspace resolver configuration at all — name resolution goes through netd,
// which a binary built without cgo cannot consult — so the absence is permanent
// and platform-wide, not a misconfigured host. A Termux install that does have
// a resolv.conf of its own is still honoured; only the empty-handed case is
// reclassified.
func (linuxProvider) SystemResolvers() ([]string, error) {
	out, err := resolvConfNameservers()
	if err != nil && runtime.GOOS == "android" {
		return nil, ErrUnsupported
	}
	return out, err
}
