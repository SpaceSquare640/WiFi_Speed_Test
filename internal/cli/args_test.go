package cli

import (
	"errors"
	"io/fs"
	"reflect"
	"testing"
	"time"
)

// fakeInfo describes a file without needing one. A real temporary file would
// make these tests pass only where an execute bit means something, which is
// not Windows, and the correction has to be reasoned about everywhere.
type fakeInfo struct{ mode fs.FileMode }

func (f fakeInfo) Name() string       { return "" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

// statter answers for a fixed set of paths and reports every other one missing.
func statter(files map[string]fs.FileMode) func(string) (fs.FileInfo, error) {
	return func(name string) (fs.FileInfo, error) {
		mode, ok := files[name]
		if !ok {
			return nil, errors.New("no such file")
		}
		return fakeInfo{mode: mode}, nil
	}
}

// The correction only ever fires on one platform, so these stand in for the
// device: they describe the argument vector a loader produces and assert that
// the flags survive it.
func TestNormaliseArgs(t *testing.T) {
	const prog = "/data/data/com.termux/files/home/wt/wifitest"

	files := statter(map[string]fs.FileMode{
		prog:            0o700,
		"./wifitest":    0o700,
		"/etc/hosts":    0o644,
		"/var/log":      fs.ModeDir | 0o755,
		"/usr/bin/true": 0o755,
	})

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
			// What the loader hands over: the program's path in front of the
			// arguments the user actually typed.
			name: "run through a loader",
			argv: []string{"anything at all", prog, "--json", "--no-history"},
			want: []string{"--json", "--no-history"},
		},
		{
			name: "run through a loader with no flags",
			argv: []string{"anything at all", prog},
			want: []string{},
		},
		{
			// A path to something that is not executable was a mistake, and the
			// parser reporting it is more use than this quietly eating it.
			name: "a readable file is still a mistake",
			argv: []string{"./wifitest", "/etc/hosts"},
			want: []string{"/etc/hosts"},
		},
		{
			name: "a directory is still a mistake",
			argv: []string{"./wifitest", "/var/log"},
			want: []string{"/var/log"},
		},
		{
			name: "a path that does not exist is still a mistake",
			argv: []string{"./wifitest", "/no/such/thing"},
			want: []string{"/no/such/thing"},
		},
		{
			// A flag is never a loader's doing, whatever the filesystem says.
			name: "a flag is never skipped",
			argv: []string{"./wifitest", "--json"},
			want: []string{"--json"},
		},
		{
			name: "no arguments at all",
			argv: []string{"./wifitest"},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normaliseArgs(tc.argv, files)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normaliseArgs(%q) = %q, want %q", tc.argv, got, tc.want)
			}
		})
	}
}

// The real stat must agree that an ordinary flag is not a file, or the
// correction would eat the first flag of every run everywhere.
func TestNamesAnExecutableRejectsFlags(t *testing.T) {
	for _, arg := range []string{"--json", "-v", "", "-"} {
		if namesAnExecutable(arg, statter(map[string]fs.FileMode{arg: 0o700})) {
			t.Errorf("%q was treated as a path left by a loader", arg)
		}
	}
}
