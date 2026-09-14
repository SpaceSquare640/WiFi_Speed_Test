//go:build windows

package latency

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isRefused reports whether the peer actively rejected the connection, which
// proves a packet reached it and an answer came back.
//
// Windows reports these through the Winsock error space rather than the POSIX
// one, so the POSIX constants would never match here.
func isRefused(err error) bool {
	return errors.Is(err, windows.WSAECONNREFUSED) || errors.Is(err, windows.WSAECONNRESET)
}
