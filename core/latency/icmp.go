package latency

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// ErrICMPUnavailable reports that ICMP probing could not be set up on this
// host, which on Windows and Android almost always means the process lacks the
// privilege. The caller should fall back to MethodTCP rather than give up.
var ErrICMPUnavailable = errors.New("latency: ICMP is not available to this process (try the default TCP method)")

// icmpProbe times an ICMP echo exchange with the target.
//
// Two socket kinds are attempted. The datagram kind needs no privilege but is
// only offered by Linux and macOS; the raw kind works everywhere but requires
// elevation. Trying both is what lets an unprivileged Linux user use --icmp at
// all, and what makes the failure on Windows explicit rather than silent.
func icmpProbe(ctx context.Context, target string, o Options) (time.Duration, outcome, error) {
	ip, err := resolveIP(ctx, target)
	if err != nil {
		return 0, fatal, err
	}

	v6 := ip.To4() == nil
	networks := []string{"udp4", "ip4:icmp"}
	if v6 {
		networks = []string{"udp6", "ip6:ipv6-icmp"}
	}

	var conn *icmp.PacketConn
	var lastErr error
	for _, network := range networks {
		conn, lastErr = icmp.ListenPacket(network, listenAddress(network))
		if lastErr == nil {
			defer conn.Close()
			break
		}
		conn = nil
	}
	if conn == nil {
		return 0, fatal, fmt.Errorf("%w: %v", ErrICMPUnavailable, lastErr)
	}

	msgType := icmp.Type(ipv4.ICMPTypeEcho)
	if v6 {
		msgType = ipv6.ICMPTypeEchoRequest
	}
	id := os.Getpid() & 0xffff
	msg := icmp.Message{
		Type: msgType,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: 1, Data: []byte("wifitest")},
	}
	encoded, err := msg.Marshal(nil)
	if err != nil {
		return 0, fatal, err
	}

	deadline := time.Now().Add(o.Timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, fatal, err
	}

	// A datagram socket addresses the peer as a UDP endpoint even though the
	// payload is ICMP; a raw socket addresses it as a bare IP.
	var dst net.Addr = &net.IPAddr{IP: ip}
	if conn.IPv4PacketConn() == nil && conn.IPv6PacketConn() == nil {
		dst = &net.UDPAddr{IP: ip}
	}

	clock := startTimer()
	if _, err := conn.WriteTo(encoded, dst); err != nil {
		return 0, classify(err), err
	}

	reply := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(reply)
		if err != nil {
			return 0, classify(err), err
		}
		elapsed := clock.elapsed()

		proto := ipv4.ICMPTypeEchoReply.Protocol()
		if v6 {
			proto = ipv6.ICMPTypeEchoReply.Protocol()
		}
		parsed, err := icmp.ParseMessage(proto, reply[:n])
		if err != nil {
			continue
		}
		// Other traffic can arrive on the same socket; only this exchange's
		// echo reply answers the question being asked.
		echo, ok := parsed.Body.(*icmp.Echo)
		if !ok || echo.Seq != 1 {
			continue
		}
		if parsed.Type != ipv4.ICMPTypeEchoReply && parsed.Type != ipv6.ICMPTypeEchoReply {
			continue
		}
		return elapsed, replied, nil
	}
}

// listenAddress returns the wildcard address for the socket family.
func listenAddress(network string) string {
	switch network {
	case "udp4", "ip4:icmp":
		return "0.0.0.0"
	default:
		return "::"
	}
}

func resolveIP(ctx context.Context, target string) (net.IP, error) {
	if ip := net.ParseIP(target); ip != nil {
		return ip, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, target)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, &net.DNSError{Err: "no addresses", Name: target}
	}
	return addrs[0].IP, nil
}
