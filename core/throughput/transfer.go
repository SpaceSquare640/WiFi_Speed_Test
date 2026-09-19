package throughput

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// uploadRequestSize is how much one POST carries.
//
// The request declares a Content-Length rather than streaming without one:
// chunked bodies of unbounded length are refused or buffered whole by enough
// servers that the measurement would be about the server, not the link. When
// the window outlasts one request the loop simply sends another.
const uploadRequestSize = 8 << 20

// transferResult is what one direction moved, and the span the body was moving
// for — which is not the same as the span the measurement took, because
// connection setup is latency and is reported separately.
type transferResult struct {
	bytes   int64
	elapsed time.Duration
}

// errNoData reports that a transfer produced nothing to divide by.
var errNoData = errors.New("throughput: no data moved")

// newClient returns the client for one measurement.
//
// A fresh transport per measurement stops one endpoint's warm connections from
// flattering the next. Three settings are deliberate:
//
//   - Compression is off. With it on the standard library counts the bytes it
//     decompressed rather than the bytes that crossed the link, so a
//     compressible body reports a rate the connection cannot deliver.
//   - HTTP/2 is off. It multiplexes every stream onto one TCP connection, which
//     would make parallel streams share a single congestion window and defeat
//     the reason for asking for them. An empty TLSNextProto is what actually
//     disables it; ForceAttemptHTTP2 alone does not.
//   - TLS verification is left alone. It is never disabled: no measurement is
//     worth teaching users to click past a certificate error.
func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DisableCompression:  true,
			MaxIdleConnsPerHost: 16,
			ForceAttemptHTTP2:   false,
			TLSNextProto:        map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
}

// run moves data one way against one URL for at most window, over the given
// number of parallel connections, and reports what moved.
func run(ctx context.Context, direction Direction, url string, streams int, window time.Duration) (transferResult, error) {
	ctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()

	client := newClient()
	defer client.CloseIdleConnections()

	var (
		mu       sync.Mutex
		total    int64
		first    time.Time
		last     time.Time
		firstErr error
	)

	var wg sync.WaitGroup
	for i := 0; i < streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, began, ended, err := streamLoop(ctx, client, direction, url)

			mu.Lock()
			defer mu.Unlock()
			total += n
			if !began.IsZero() && (first.IsZero() || began.Before(first)) {
				first = began
			}
			if ended.After(last) {
				last = ended
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}()
	}
	wg.Wait()

	if total == 0 {
		if firstErr != nil {
			return transferResult{}, firstErr
		}
		return transferResult{}, errNoData
	}
	// Spanning every stream's body window: with one stream this is that
	// stream's transfer, and with several it is the period the link was busy.
	elapsed := last.Sub(first)
	if elapsed <= 0 {
		return transferResult{}, errors.New("throughput: transfer finished too quickly to time")
	}
	return transferResult{bytes: total, elapsed: elapsed}, nil
}

// streamLoop keeps one connection busy until the window closes.
//
// Repeating the request is what makes the measurement independent of how large
// a file the endpoint happens to serve: a small one on a fast link would
// otherwise finish before the connection left slow start, and report a figure
// about TCP rather than about the link.
func streamLoop(ctx context.Context, c *http.Client, d Direction, url string) (int64, time.Time, time.Time, error) {
	var (
		total   int64
		first   time.Time
		last    time.Time
		lastErr error
	)

	for ctx.Err() == nil {
		n, began, ended, err := once(ctx, c, d, url)
		total += n
		if !began.IsZero() && (first.IsZero() || began.Before(first)) {
			first = began
		}
		if ended.After(last) {
			last = ended
		}
		if err != nil {
			lastErr = err
			// A request that failed once against a fixed URL will fail again;
			// looping would only hammer the endpoint to learn nothing.
			break
		}
	}

	if total > 0 {
		return total, first, last, nil
	}
	return 0, first, last, lastErr
}

// once performs a single request in the given direction.
func once(ctx context.Context, c *http.Client, d Direction, url string) (int64, time.Time, time.Time, error) {
	if d == DirectionUpload {
		return uploadOnce(ctx, c, url)
	}
	return downloadOnce(ctx, c, url)
}

func downloadOnce(ctx context.Context, c *http.Client, url string) (int64, time.Time, time.Time, error) {
	var zero time.Time

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, zero, zero, err
	}
	// Asked for explicitly as well as disabled on the transport, so that an
	// endpoint which compresses by default is told plainly not to.
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := c.Do(req)
	if err != nil {
		return 0, zero, zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, zero, zero, fmt.Errorf("throughput: %s answered %s", url, resp.Status)
	}

	// The clock starts once the headers are in, which is where the body begins.
	started := time.Now()
	n, err := io.Copy(io.Discard, resp.Body)
	ended := time.Now()

	// The window closing mid-body is the design, not a failure: what arrived
	// crossed the link and counts.
	if err != nil && n > 0 && ctx.Err() != nil {
		err = nil
	}
	return n, started, ended, err
}

func uploadOnce(ctx context.Context, c *http.Client, url string) (int64, time.Time, time.Time, error) {
	body := newPayload(uploadRequestSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return 0, time.Time{}, time.Time{}, err
	}
	// Declared so the server sees a normal sized request rather than a chunked
	// stream of unknown length.
	req.ContentLength = uploadRequestSize
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.Do(req)
	if resp != nil {
		// Drained before closing so the connection can be reused by the next
		// turn of the loop instead of being torn down and dialled again.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	sent, started, ended := body.stats()
	if err != nil {
		if sent > 0 && ctx.Err() != nil {
			return sent, started, ended, nil
		}
		return sent, started, ended, err
	}
	if resp != nil && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return sent, started, ended, fmt.Errorf("throughput: %s answered %s", url, resp.Status)
	}
	return sent, started, ended, nil
}
