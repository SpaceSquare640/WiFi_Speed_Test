// Package netinfo discovers the host's network configuration: the default
// gateway and the system resolvers. These become the targets of the first and
// second diagnostic layers, so that layered diagnostics need no user setup.
//
// Implementations must use native system calls or the standard library. Parsing
// the textual output of ipconfig or ip route is explicitly forbidden: that
// output is localised, and the previous generation of this tool failed on
// non-English Windows installations for exactly that reason.
package netinfo

import (
	"errors"
	"fmt"
)

// ErrNotAvailable reports that the value could not be determined on this host.
// Callers should degrade gracefully — a layer whose target is unknown is
// skipped, never guessed.
var ErrNotAvailable = errors.New("netinfo: value not available on this host")

// ErrUnsupported reports that the platform cannot supply the value at all, as
// distinct from a host that merely happens not to have it configured. The
// difference matters to what the user is told: a missing value is worth
// investigating, whereas a platform that never had one is not a fault and
// should not be presented as a failed measurement.
//
// It wraps ErrNotAvailable so that callers testing for availability alone need
// no change.
var ErrUnsupported = fmt.Errorf("%w: not supported on this platform", ErrNotAvailable)

// Provider exposes host network facts. It is an interface so that tests can
// supply fixtures and so that each platform can register its own implementation.
type Provider interface {
	// DefaultGateway returns the IP address of the default route's next hop.
	DefaultGateway() (string, error)

	// SystemResolvers returns the resolver addresses configured for this host,
	// most preferred first.
	SystemResolvers() ([]string, error)
}

// Default returns the Provider for the current platform.
//
// Every platform returns a usable Provider; one that cannot answer on this host
// reports ErrNotAvailable per call rather than being absent, so callers have a
// single code path.
func Default() Provider { return platformProvider() }

// Static is a Provider with fixed answers, for tests and for callers that have
// already resolved these facts by other means.
type Static struct {
	Gateway   string
	Resolvers []string
}

// DefaultGateway implements Provider.
func (s Static) DefaultGateway() (string, error) {
	if s.Gateway == "" {
		return "", ErrNotAvailable
	}
	return s.Gateway, nil
}

// SystemResolvers implements Provider.
func (s Static) SystemResolvers() ([]string, error) {
	if len(s.Resolvers) == 0 {
		return nil, ErrNotAvailable
	}
	out := make([]string, len(s.Resolvers))
	copy(out, s.Resolvers)
	return out, nil
}

// unsupported answers ErrNotAvailable for every question. It backs platforms
// with no implementation, so that a build for an unforeseen target still runs
// and simply skips the layers it cannot resolve.
type unsupported struct{}

func (unsupported) DefaultGateway() (string, error)    { return "", ErrNotAvailable }
func (unsupported) SystemResolvers() ([]string, error) { return nil, ErrNotAvailable }
