package cli

import (
	"errors"
	"reflect"
	"testing"
)

// The correction only ever fires on one platform, so these tests stand in for
// the device: they describe the argument vector Termux's loader produces and
// assert that the flags survive it.
func TestNormaliseArgs(t *testing.T) {
	const exe = "/data/data/com.termux/files/home/wt/wifitest"

	// Two paths name the same file when either is this program. Nothing here
	// touches the disk; the identity is the thing being described.
	same := func(a, b string) bool {
		names := map[string]bool{exe: true, "./wifitest": true, "wifitest": true}
		return names[a] && names[b]
	}
	found := func() (string, error) { return exe, nil }

	tests := []struct {
		name string
		argv []string
		want []string
	}{
		{
			name: "run normally",
			argv: []string{"./wifitest", "--json", "--no-history"},
			want: []string{"--json", "--no-history"},
		},
		{
			// What Termux's loader hands over: the linker first, this program
			// second, and the flags only after that.
			name: "run through the loader",
			argv: []string{"/system/bin/linker64", exe, "--json", "--no-history"},
			want: []string{"--json", "--no-history"},
		},
		{
			name: "run through the loader with no flags",
			argv: []string{"/system/bin/linker64", exe},
			want: []string{},
		},
		{
			// A path that is not this program is a genuine stray argument and
			// must still reach the parser, which rejects it.
			name: "a stray argument is left alone",
			argv: []string{"./wifitest", "/etc/hosts"},
			want: []string{"/etc/hosts"},
		},
		{
			name: "no arguments at all",
			argv: []string{"./wifitest"},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normaliseArgs(tc.argv, found, same)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normaliseArgs(%q) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}

// Guessing would be worse than not correcting: the ordinary reading is right
// everywhere the correction is not needed, which is every platform but one.
func TestNormaliseArgsWithoutKnowingItsOwnPath(t *testing.T) {
	missing := func() (string, error) { return "", errors.New("no /proc/self/exe") }
	never := func(a, b string) bool { return false }

	got := normaliseArgs([]string{"./wifitest", "--json"}, missing, never)
	if !reflect.DeepEqual(got, []string{"--json"}) {
		t.Errorf("normaliseArgs = %q, want [--json]", got)
	}
}

// The real comparison must agree with itself, or the correction would fire on
// every ordinary run and eat the first flag.
func TestSameFileRecognisesOneFile(t *testing.T) {
	if !sameFile(".", ".") {
		t.Error("sameFile says the working directory is not itself")
	}
	if sameFile(".", "no-such-path-here") {
		t.Error("sameFile matched a path that does not exist")
	}
}
