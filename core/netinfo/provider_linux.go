//go:build linux

package netinfo

import (
	"encoding/binary"
	"net/netip"
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
func (linuxProvider) DefaultGateway() (string, error) {
	rib, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_UNSPEC)
	if err != nil {
		return "", ErrNotAvailable
	}
	msgs, err := syscall.ParseNetlinkMessage(rib)
	if err != nil {
		return "", ErrNotAvailable
	}

	type route struct {
		addr     netip.Addr
		priority uint32
	}
	var routes []route

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
		if rt.Table != syscall.RT_TABLE_MAIN {
			continue
		}
		if rt.Type != syscall.RTN_UNICAST {
			continue
		}

		attrs, err := syscall.ParseNetlinkRouteAttr(&m)
		if err != nil {
			continue
		}
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
			}
		}
		if gw.IsValid() && !gw.IsUnspecified() {
			routes = append(routes, route{addr: gw, priority: priority})
		}
	}
	if len(routes) == 0 {
		return "", ErrNotAvailable
	}

	// IPv4 first, then the lowest metric, matching the kernel's own preference.
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].addr.Is4() != routes[j].addr.Is4() {
			return routes[i].addr.Is4()
		}
		return routes[i].priority < routes[j].priority
	})
	return routes[0].addr.Unmap().String(), nil
}

func (linuxProvider) SystemResolvers() ([]string, error) { return resolvConfNameservers() }
