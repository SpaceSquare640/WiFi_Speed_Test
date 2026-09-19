package throughput

import (
	"crypto/rand"
	"io"
	"sync"
	"time"
)

// payloadBlockSize is the size of the random block an upload body repeats.
//
// It is far larger than the 32 KiB window DEFLATE slides, so a compressor
// anywhere on the path only ever sees unrepeated random bytes and cannot shrink
// the stream. This is not fussiness: sending zeroes instead would let any gzip
// on the path compress a megabyte to nothing and report an upload rate tens of
// times what the link can carry.
const payloadBlockSize = 256 << 10

var (
	payloadOnce  sync.Once
	payloadBlock []byte
)

// randomBlock returns the process-wide block that upload bodies repeat.
//
// It is generated once. Filling it per request would measure how fast this
// machine can produce random bytes rather than how fast the link carries them,
// which on a fast connection is the difference between the two figures.
func randomBlock() []byte {
	payloadOnce.Do(func() {
		payloadBlock = make([]byte, payloadBlockSize)
		// A short read would still leave incompressible bytes at the front and
		// the block is not a secret, so there is no failure worth handling.
		_, _ = rand.Read(payloadBlock)
	})
	return payloadBlock
}

// payloadReader is the body of one upload request: a fixed number of bytes
// drawn from the repeating random block, produced without ever holding them all
// in memory.
//
// It also records when the body started and finished moving. The HTTP client
// opens the connection and writes the headers before it reads the first byte,
// so the first Read is the closest observable moment to the body reaching the
// wire — and timing from there keeps a handshake, which is latency, out of a
// throughput figure.
//
// The transport writes the request from its own goroutine, so every field is
// guarded: without the lock the race detector is right to complain.
type payloadReader struct {
	mu        sync.Mutex
	remaining int64
	offset    int

	sent  int64
	first time.Time
	last  time.Time
}

// newPayload returns a body that will yield size bytes.
func newPayload(size int64) *payloadReader { return &payloadReader{remaining: size} }

// Read implements io.Reader.
func (p *payloadReader) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.first.IsZero() {
		p.first = time.Now()
	}
	if p.remaining <= 0 {
		return 0, io.EOF
	}

	block := randomBlock()
	n := len(b)
	if int64(n) > p.remaining {
		n = int(p.remaining)
	}
	if n > len(block)-p.offset {
		n = len(block) - p.offset
	}
	copy(b[:n], block[p.offset:p.offset+n])

	p.offset = (p.offset + n) % len(block)
	p.remaining -= int64(n)
	p.sent += int64(n)
	p.last = time.Now()
	return n, nil
}

// stats reports what left the machine and the window it left in. A request that
// was cancelled part way still moved the bytes it moved, so this is read after
// a failure as well as after a success.
func (p *payloadReader) stats() (sent int64, first, last time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sent, p.first, p.last
}
