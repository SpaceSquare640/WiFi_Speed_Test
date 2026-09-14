//go:build linux || darwin

package netinfo

import (
	"strings"
	"testing"
)

func TestParseResolvConf(t *testing.T) {
	const input = `
# a comment
domain example.invalid
nameserver 192.0.2.1
nameserver 192.0.2.1
nameserver 2001:db8::1%eth0   ; trailing comment
nameserver not-an-address
options edns0
nameserver
`

	got, err := parseResolvConf(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseResolvConf: %v", err)
	}

	want := []string{"192.0.2.1", "2001:db8::1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
