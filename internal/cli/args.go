package cli

import (
	"io/fs"
	"os"
	"strings"
)

// Args returns the command line to parse, corrected for an invocation made
// through a dynamic loader rather than directly by the kernel.
//
// Android has forbidden executing a file from an application's own data
// directory since Android 10. Termux works around it by handing the binary to
// the system linker instead of exec'ing it:
//
//	execve("/system/bin/linker64", ["…", "/path/to/prog", …the real arguments])
//
// A dynamically linked program never notices, because the linker rewrites the
// stack before handing over. This program is a static PIE — it has no C
// dependency to link against — so the linker loads it and jumps to its entry
// with the stack untouched. The runtime then reads an argument vector that
// still carries the path the linker was given, and every real argument sits one
// place further along than the parser looks: the first flag reads as a stray
// path, parsing stops, and nothing the user asked for is honoured.
func Args() []string { return normaliseArgs(os.Args, os.Stat) }

// normaliseArgs drops a leading argument that a loader left in front of the
// real ones.
//
// The rule deliberately assumes nothing about the shape of the vector, because
// an earlier attempt that did — matching the program's own path against
// os.Executable — was built on a guess about which entry held what, and the
// guess was wrong on the one platform it existed for.
//
// What can be relied on instead is the command itself: it takes no positional
// arguments at all, so a leading entry that is not a flag was never something a
// user could have meant. It is skipped only when it names a file that exists
// and carries an execute bit, which is what a loader would have been passed.
// Anything else — a mistyped flag, a path to a document — still reaches the
// parser and is still reported, because being told about a mistake is more use
// than having it quietly ignored.
func normaliseArgs(argv []string, stat func(string) (fs.FileInfo, error)) []string {
	if len(argv) < 2 {
		return nil
	}
	if namesAnExecutable(argv[1], stat) {
		return argv[2:]
	}
	return argv[1:]
}

// namesAnExecutable reports whether an argument points at a file that could
// have been handed to a loader.
func namesAnExecutable(arg string, stat func(string) (fs.FileInfo, error)) bool {
	if arg == "" || strings.HasPrefix(arg, "-") {
		return false
	}
	info, err := stat(arg)
	if err != nil {
		return false
	}
	mode := info.Mode()
	return mode.IsRegular() && mode.Perm()&0o111 != 0
}
