package netinfo

import "testing"

func TestStaticProvider(t *testing.T) {
	var empty Static
	if _, err := empty.DefaultGateway(); err != ErrNotAvailable {
		t.Errorf("empty gateway: got %v, want ErrNotAvailable", err)
	}
	if _, err := empty.SystemResolvers(); err != ErrNotAvailable {
		t.Errorf("empty resolvers: got %v, want ErrNotAvailable", err)
	}

	s := Static{Gateway: "192.0.2.254", Resolvers: []string{"192.0.2.1"}}
	gw, err := s.DefaultGateway()
	if err != nil || gw != "192.0.2.254" {
		t.Errorf("gateway: got %q, %v", gw, err)
	}

	// The caller must not be able to reach into the provider's own slice.
	out, _ := s.SystemResolvers()
	out[0] = "mutated"
	again, _ := s.SystemResolvers()
	if again[0] != "192.0.2.1" {
		t.Errorf("SystemResolvers aliased its backing slice: %q", again[0])
	}
}
