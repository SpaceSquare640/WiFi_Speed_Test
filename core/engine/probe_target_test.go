package engine

import (
	"testing"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

func TestProbeTarget(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []throughput.Endpoint
		wantHost  string
		wantPort  int
	}{
		{
			name:      "https defaults to 443",
			endpoints: []throughput.Endpoint{{DownloadURL: "https://speed.example.com/down"}},
			wantHost:  "speed.example.com",
			wantPort:  443,
		},
		{
			name:      "http defaults to 80",
			endpoints: []throughput.Endpoint{{DownloadURL: "http://speed.example.com/down"}},
			wantHost:  "speed.example.com",
			wantPort:  80,
		},
		{
			name:      "an explicit port wins over the scheme",
			endpoints: []throughput.Endpoint{{DownloadURL: "http://speed.example.com:8080/down"}},
			wantHost:  "speed.example.com",
			wantPort:  8080,
		},
		{
			// An upload-only endpoint still names a host worth timing.
			name:      "falls back to the upload URL",
			endpoints: []throughput.Endpoint{{UploadURL: "https://up.example.com/up"}},
			wantHost:  "up.example.com",
			wantPort:  443,
		},
		{
			name: "skips an endpoint that names no host",
			endpoints: []throughput.Endpoint{
				{DownloadURL: "not a url"},
				{DownloadURL: "https://second.example.com/down"},
			},
			wantHost: "second.example.com",
			wantPort: 443,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, ports, ok := probeTarget(tc.endpoints)
			if !ok {
				t.Fatal("probeTarget reported no target")
			}
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			if len(ports) != 1 || ports[0] != tc.wantPort {
				t.Errorf("ports = %v, want [%d]", ports, tc.wantPort)
			}
		})
	}
}

func TestProbeTargetWithNothingToTime(t *testing.T) {
	cases := map[string][]throughput.Endpoint{
		"no endpoints":  nil,
		"no URLs":       {{Name: "named but empty"}},
		"no host in it": {{DownloadURL: "not a url"}},
	}
	for name, endpoints := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, ok := probeTarget(endpoints); ok {
				t.Error("probeTarget claimed a target it cannot have found")
			}
		})
	}
}
