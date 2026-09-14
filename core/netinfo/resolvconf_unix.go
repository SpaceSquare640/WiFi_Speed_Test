//go:build linux || darwin

package netinfo

import (
	"bufio"
	"net/netip"
	"os"
	"strings"
)

// resolvConfPaths are searched in order. The Termux path comes first when
// PREFIX is set, because a Termux install has its own tree and the system
// /etc/resolv.conf may not exist or may not be readable there.
func resolvConfPaths() []string {
	var paths []string
	if prefix := os.Getenv("PREFIX"); prefix != "" {
		paths = append(paths, prefix+"/etc/resolv.conf")
	}
	return append(paths, "/etc/resolv.conf")
}

// resolvConfNameservers returns the configured resolvers, most preferred first.
//
// resolv.conf is a fixed-format configuration file, not command output: its
// keywords are never localised, so reading it does not run the risk the package
// documentation warns about.
func resolvConfNameservers() ([]string, error) {
	for _, path := range resolvConfPaths() {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		out, err := parseResolvConf(f)
		f.Close()
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}
	return nil, ErrNotAvailable
}

// parseResolvConf extracts nameserver addresses in file order.
func parseResolvConf(r interface{ Read([]byte) (int, error) }) ([]string, error) {
	var out []string
	seen := make(map[netip.Addr]bool)

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		// A comment may follow the directive on the same line.
		if i := strings.IndexAny(line, "#;"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		// A scoped address such as fe80::1%eth0 carries a zone the probe cannot
		// use as a bare target; the zone is dropped and the address kept.
		value := fields[1]
		if i := strings.IndexByte(value, '%'); i >= 0 {
			value = value[:i]
		}
		addr, err := netip.ParseAddr(value)
		if err != nil || seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr.Unmap().String())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
