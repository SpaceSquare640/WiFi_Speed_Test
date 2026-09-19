// Package layers resolves the three diagnostic layers to concrete targets.
//
// The layering is topological, not geographic: the second layer is whatever
// resolver the host is configured to use, which in practice represents the ISP.
// This makes the design correct worldwide without a maintained region list.
package layers

import (
	"errors"
	"net/netip"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/netinfo"
)

// Kind identifies a diagnostic layer.
type Kind string

const (
	KindGateway        Kind = "gateway"
	KindRegionalEgress Kind = "regional_egress"
	KindInternational  Kind = "international"
)

// DefaultInternational is the third layer's target when the user supplies none.
//
// It is a public anycast resolver, chosen because it is reachable worldwide and
// answers on a well-known TCP port. It is a default, not a dependency: the whole
// point of the override is that no traffic need ever reach an address the user
// did not choose.
const DefaultInternational = "1.1.1.1"

// ErrNoLayers reports that not one layer could be resolved, which means the
// host has no usable network configuration at all.
var ErrNoLayers = errors.New("layers: no diagnostic layer could be resolved")

// Layer is one resolved diagnostic target.
type Layer struct {
	Kind   Kind
	Target string

	// UserDefined records that the target came from configuration rather than
	// auto-detection.
	UserDefined bool
}

// Set is the resolved collection of layers for one diagnostic run. Layers that
// could not be resolved are absent rather than zero-valued.
type Set struct {
	Layers []Layer

	// ResolverIsPublic reports that the host's configured resolver is itself a
	// well-known public address, which collapses the regional layer onto the
	// international one. The result must say so, or the user will misread two
	// identical rows as corroboration.
	ResolverIsPublic bool

	// RegionalUnsupported reports that the regional layer is missing because
	// the platform has no resolver configuration to read, not because this host
	// is misconfigured. Without it the row simply vanishes and the user is left
	// to guess whether the layer failed or was never attempted.
	RegionalUnsupported bool
}

// ProbePorts returns the TCP ports worth trying for this layer, in order.
//
// The layer kind is what determines them, because the kind is what says which
// sort of machine the target is. A gateway is a router, and which port a router
// answers on varies by model; the other two layers are resolvers, which answer
// on the DNS port.
func (l Layer) ProbePorts() []int {
	switch l.Kind {
	case KindGateway:
		return []int{80, 443, 53}
	default:
		return []int{53, 443, 80}
	}
}

// Get returns the layer of the given kind, if it was resolved.
func (s Set) Get(k Kind) (Layer, bool) {
	for _, l := range s.Layers {
		if l.Kind == k {
			return l, true
		}
	}
	return Layer{}, false
}

// Overrides carries user-supplied targets. An empty field means auto-detect.
type Overrides struct {
	Gateway        string
	RegionalEgress string
	International  string
}

// Resolve determines the targets for this run.
func Resolve(o Overrides) (Set, error) { return ResolveWith(netinfo.Default(), o) }

// ResolveWith determines the targets using a specific Provider. It exists so
// that callers holding their own Provider — and tests holding a fixture — do not
// have to reach for the platform one.
//
// A layer that cannot be resolved is omitted. Only a completely empty set is an
// error: one unreachable layer is a finding, no layers at all is a broken host.
func ResolveWith(p netinfo.Provider, o Overrides) (Set, error) {
	var set Set

	if target, ok := normalise(o.Gateway); ok {
		set.Layers = append(set.Layers, Layer{Kind: KindGateway, Target: target, UserDefined: true})
	} else if gw, err := p.DefaultGateway(); err == nil {
		if target, ok := normalise(gw); ok {
			set.Layers = append(set.Layers, Layer{Kind: KindGateway, Target: target})
		}
	}

	// The regional layer stands for the ISP. The host's own resolver is the most
	// reliable address that plays that role without a maintained region list.
	var regional string
	if target, ok := normalise(o.RegionalEgress); ok {
		regional = target
		set.Layers = append(set.Layers, Layer{Kind: KindRegionalEgress, Target: target, UserDefined: true})
	} else if resolvers, err := p.SystemResolvers(); err == nil {
		for _, r := range resolvers {
			if target, ok := normalise(r); ok {
				regional = target
				set.Layers = append(set.Layers, Layer{Kind: KindRegionalEgress, Target: target})
				break
			}
		}
	} else if errors.Is(err, netinfo.ErrUnsupported) {
		set.RegionalUnsupported = true
	}

	international := DefaultInternational
	userDefined := false
	if target, ok := normalise(o.International); ok {
		international = target
		userDefined = true
	}
	set.Layers = append(set.Layers, Layer{Kind: KindInternational, Target: international, UserDefined: userDefined})

	// Two identical rows read as corroboration unless the collapse is stated.
	set.ResolverIsPublic = regional != "" && (regional == international || IsPublicResolver(regional))

	if len(set.Layers) == 0 {
		return Set{}, ErrNoLayers
	}
	return set, nil
}

// publicResolvers are the addresses that, when configured as the host's
// resolver, mean the regional layer is not measuring the ISP at all.
//
// The list does not have to be exhaustive to be useful: missing an entry costs
// a caveat the user does not see, never a wrong measurement.
var publicResolvers = map[string]struct{}{
	"1.1.1.1":              {}, // Cloudflare
	"1.0.0.1":              {},
	"2606:4700:4700::1111": {},
	"2606:4700:4700::1001": {},
	"8.8.8.8":              {}, // Google
	"8.8.4.4":              {},
	"2001:4860:4860::8888": {},
	"2001:4860:4860::8844": {},
	"9.9.9.9":              {}, // Quad9
	"149.112.112.112":      {},
	"2620:fe::fe":          {},
	"208.67.222.222":       {}, // OpenDNS
	"208.67.220.220":       {},
	"94.140.14.14":         {}, // AdGuard
	"94.140.15.15":         {},
}

// IsPublicResolver reports whether target is a well-known public resolver.
func IsPublicResolver(target string) bool {
	normalised, ok := normalise(target)
	if !ok {
		return false
	}
	_, found := publicResolvers[normalised]
	return found
}

// normalise validates a target and returns it in canonical form.
//
// Addresses that cannot serve as a probe target are rejected outright: the
// unspecified address goes nowhere, and loopback would measure the host talking
// to itself and report perfect health for a network that is down.
func normalise(target string) (string, bool) {
	if target == "" {
		return "", false
	}
	addr, err := netip.ParseAddr(target)
	if err != nil {
		// A hostname is accepted as given; the probe resolves it.
		return target, true
	}
	addr = addr.Unmap()
	if addr.IsUnspecified() || addr.IsLoopback() {
		return "", false
	}
	// The zone is dropped: it is meaningful to the host, not to a probe target.
	return addr.WithZone("").String(), true
}
