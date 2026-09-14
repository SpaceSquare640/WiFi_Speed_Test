//go:build !windows

package latency

import (
	"errors"
	"syscall"
)

// isRefused reports whether the peer actively rejected the connection, which
// proves a packet reached it and an answer came back.
func isRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET)
}
