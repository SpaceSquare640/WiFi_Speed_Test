package throughput

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testWindow keeps each measurement short. Over loopback it is still long
// enough to move a great many bytes, which is all these tests need.
const testWindow = 300 * time.Millisecond

// downloadServer serves a fixed block repeatedly and records what it was asked
// for, so a test can check the request as well as the result.
func downloadServer(t *testing.T, seen *string) *httptest.Server {
	t.Helper()
	block := make([]byte, 512<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.Header.Get("Accept-Encoding")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(block)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func uploadServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMeasureWithoutEndpoints(t *testing.T) {
	res := Measure(context.Background(), Options{Direction: DirectionDownload})
	if !errors.Is(res.Err, ErrNoEndpoints) {
		t.Fatalf("Err = %v, want ErrNoEndpoints", res.Err)
	}
	if res.OK {
		t.Error("OK = true with no endpoints")
	}
}

func TestMeasureDownload(t *testing.T) {
	var accepted string
	srv := downloadServer(t, &accepted)

	res := Measure(context.Background(), Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{{Name: "local", DownloadURL: srv.URL}},
		Timeout:   testWindow,
	})

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
	if !res.OK {
		t.Fatal("OK = false")
	}
	if res.Mbps <= 0 {
		t.Errorf("Mbps = %v, want above zero", res.Mbps)
	}
	if len(res.Samples) != 1 {
		t.Fatalf("got %d samples, want 1", len(res.Samples))
	}
	// Counting decompressed bytes would report a rate the link never carried,
	// so the request has to ask for none.
	if accepted != "identity" {
		t.Errorf("Accept-Encoding = %q, want \"identity\"", accepted)
	}
}

func TestMeasureUpload(t *testing.T) {
	srv := uploadServer(t)

	res := Measure(context.Background(), Options{
		Direction: DirectionUpload,
		Endpoints: []Endpoint{{Name: "local", UploadURL: srv.URL}},
		Timeout:   testWindow,
	})

	if res.Err != nil {
		t.Fatalf("Err = %v, want nil", res.Err)
	}
	if !res.OK || res.Mbps <= 0 {
		t.Errorf("OK = %v, Mbps = %v; want true and above zero", res.OK, res.Mbps)
	}
}

func TestMeasureWhenNoEndpointServesTheDirection(t *testing.T) {
	// A download-only endpoint is a normal configuration, not a fault. Asking
	// it for an upload is the same situation as configuring nothing at all, so
	// it reports that rather than inventing a failed measurement.
	res := Measure(context.Background(), Options{
		Direction: DirectionUpload,
		Endpoints: []Endpoint{{Name: "download only", DownloadURL: "https://example.invalid/down"}},
		Timeout:   testWindow,
	})

	if !errors.Is(res.Err, ErrNoEndpoints) {
		t.Fatalf("Err = %v, want it to wrap ErrNoEndpoints", res.Err)
	}
	if len(res.Samples) != 0 {
		t.Errorf("got %d samples, want none — an endpoint meant for the other direction has not failed", len(res.Samples))
	}
	// The advice is the part the user needs, so the message must carry it.
	if !strings.Contains(res.Err.Error(), "--upload-endpoint") {
		t.Errorf("Err = %q, want it to name --upload-endpoint", res.Err)
	}
}

func TestMeasureIgnoresEndpointsForTheOtherDirection(t *testing.T) {
	srv := downloadServer(t, nil)

	// A list holding both kinds is the ordinary case once download and upload
	// live on different paths. The download run must simply not see the other.
	res := Measure(context.Background(), Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{
			{Name: "upload only", UploadURL: "https://example.invalid/up"},
			{Name: "download", DownloadURL: srv.URL},
		},
		Timeout: testWindow,
	})

	if !res.OK {
		t.Fatalf("OK = false, Err = %v", res.Err)
	}
	if len(res.Samples) != 1 {
		t.Fatalf("got %d samples, want 1 — the upload-only endpoint must not appear", len(res.Samples))
	}
	if res.Samples[0].Endpoint != "download" {
		t.Errorf("sample endpoint = %q, want \"download\"", res.Samples[0].Endpoint)
	}
}

func TestMeasureReportsTheStreamCountItUsed(t *testing.T) {
	srv := downloadServer(t, nil)

	res := Measure(context.Background(), Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{{Name: "local", DownloadURL: srv.URL}},
		Streams:   2,
		Timeout:   testWindow,
	})
	if res.Streams != 2 {
		t.Errorf("Streams = %d, want 2 — the figure means a different thing at each count", res.Streams)
	}

	res = Measure(context.Background(), Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{{Name: "local", DownloadURL: srv.URL}},
		Timeout:   testWindow,
	})
	if res.Streams != DefaultStreams {
		t.Errorf("Streams = %d, want the default %d", res.Streams, DefaultStreams)
	}
}

func TestMeasureRecordsEachEndpointThatFailed(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer refusing.Close()

	res := Measure(context.Background(), Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{
			{Name: "one", DownloadURL: refusing.URL},
			{Name: "two", DownloadURL: refusing.URL},
		},
		Timeout: testWindow,
	})

	if !errors.Is(res.Err, ErrAllEndpointsFailed) {
		t.Fatalf("Err = %v, want ErrAllEndpointsFailed", res.Err)
	}
	if len(res.Samples) != 2 {
		t.Fatalf("got %d samples, want 2 — a failing endpoint must not stop the rest", len(res.Samples))
	}
	for i, s := range res.Samples {
		if s.Err == nil {
			t.Errorf("sample %d: want an error explaining the failure", i)
		}
	}
}

func TestMeasureStopsAtSamples(t *testing.T) {
	srv := downloadServer(t, nil)

	res := Measure(context.Background(), Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{
			{Name: "one", DownloadURL: srv.URL},
			{Name: "two", DownloadURL: srv.URL},
			{Name: "three", DownloadURL: srv.URL},
		},
		Samples: 2,
		Timeout: testWindow,
	})

	if !res.OK {
		t.Fatalf("OK = false, Err = %v", res.Err)
	}
	if len(res.Samples) != 2 {
		t.Errorf("got %d samples, want 2 — --servers bounds the work done", len(res.Samples))
	}
}

func TestMeasureHonoursCancellation(t *testing.T) {
	srv := downloadServer(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := Measure(ctx, Options{
		Direction: DirectionDownload,
		Endpoints: []Endpoint{{Name: "local", DownloadURL: srv.URL}},
		Timeout:   testWindow,
	})
	if res.OK {
		t.Error("OK = true after the context was cancelled")
	}
}

func TestRate(t *testing.T) {
	// 1,250,000 bytes in one second is 10 Mbps in the decimal units link rates
	// are sold in.
	got := rate(transferResult{bytes: 1_250_000, elapsed: time.Second})
	if got < 9.99 || got > 10.01 {
		t.Errorf("rate = %v, want 10", got)
	}
	if got := rate(transferResult{bytes: 100, elapsed: 0}); got != 0 {
		t.Errorf("rate with no elapsed time = %v, want 0", got)
	}
}

func TestPayloadYieldsExactlyTheRequestedSize(t *testing.T) {
	const size = 700 << 10 // deliberately not a multiple of the block

	p := newPayload(size)
	n, err := io.Copy(io.Discard, p)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if n != size {
		t.Errorf("copied %d bytes, want %d", n, size)
	}

	sent, first, last := p.stats()
	if sent != size {
		t.Errorf("stats sent = %d, want %d", sent, size)
	}
	if first.IsZero() || last.IsZero() {
		t.Error("stats must timestamp the body, since throughput is timed from them")
	}
	if last.Before(first) {
		t.Error("the body cannot finish before it starts")
	}
}

func TestPayloadIsIncompressible(t *testing.T) {
	// Zeroes would let any gzip on the path compress the body to nothing and
	// report an upload rate tens of times the truth. This is the guard.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := io.Copy(zw, newPayload(payloadBlockSize)); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	if ratio := float64(buf.Len()) / float64(payloadBlockSize); ratio < 0.9 {
		t.Errorf("gzip shrank the body to %.0f%% of its size; it must stay incompressible", ratio*100)
	}
}
