package cli

import "os"

// Args returns the command line to parse, corrected for an invocation made
// through a dynamic loader rather than directly by the kernel.
//
// Android has forbidden executing a file from an application's own data
// directory since Android 10. Termux works around it by handing the binary to
// the system linker instead of exec'ing it:
//
//	execve("/system/bin/linker64", ["/system/bin/linker64", "/path/to/prog", …])
//
// A dynamically linked program never notices, because the linker rewrites the
// stack before handing over. This program is a static PIE — it has no C
// dependency to link against — so the linker simply loads it and jumps to its
// entry with the stack untouched. The runtime then reads an argument vector
// that still begins with the linker's own path, and every argument arrives one
// place further along than it should: the first flag looks like a stray path,
// parsing stops there, and nothing the user asked for is read.
func Args() []string { return normaliseArgs(os.Args, os.Executable, sameFile) }

// sameFile reports whether two paths name the same file on disk, which survives
// the symlinks and /proc paths that a string comparison would not.
func sameFile(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

// normaliseArgs drops the program's own path from the front of the arguments
// when a loader left it there.
//
// The test is deliberately narrow: the first entry must not be this program
// while the second one is. Nothing but a loader invocation produces that
// pairing — a program run normally has itself first — and because this command
// accepts no positional arguments at all, being wrong costs an error message
// rather than a misread instruction.
//
// The lookup and the comparison are parameters so that the correction can be
// tested on the platforms that never need it, which is all of them but one.
func normaliseArgs(argv []string, executable func() (string, error), same func(a, b string) bool) []string {
	if len(argv) < 2 {
		return nil
	}
	exe, err := executable()
	if err != nil {
		// Without knowing which file this is, the ordinary reading is the safer
		// one: it mangles nothing on every platform that needs no correction.
		return argv[1:]
	}
	if !same(argv[0], exe) && same(argv[1], exe) {
		return argv[2:]
	}
	return argv[1:]
}
