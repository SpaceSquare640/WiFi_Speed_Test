// Package history persists past reports on the local device and derives trends
// from them.
//
// Storage is local and never synchronised. Results are bound to one device on
// one network at one moment, so sharing them across devices would be noise
// dressed as insight.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/config"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
)

// TrendWindow is how many recent entries contribute to a trend summary.
const TrendWindow = 20

// DefaultLimit is how many entries are retained before the oldest are dropped.
const DefaultLimit = 1000

// flatBand is how much the latest value may differ from the one before it and
// still count as unchanged. Without a band every run would report a direction,
// and an arrow that always points somewhere tells the reader nothing.
const flatBand = 0.05

// Entry is one stored report reduced to the figures trends are drawn from.
type Entry struct {
	Timestamp    time.Time `json:"timestamp"`
	DownloadMbps float64   `json:"download_mbps"`
	UploadMbps   float64   `json:"upload_mbps"`
	LatencyMS    float64   `json:"latency_ms"`
}

// Stats summarise one series.
type Stats struct {
	Avg float64
	Min float64
	Max float64
}

// Direction describes how the latest value moved against the one before it.
type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
	DirectionFlat Direction = "flat"
)

// Trend is the summary presented alongside a fresh report.
type Trend struct {
	Count    int
	Download Stats
	Upload   Stats
	Latency  Stats

	DownloadDirection Direction
	UploadDirection   Direction
	LatencyDirection  Direction
}

// Store persists entries.
//
// A corrupt or unreadable store must degrade to an empty history rather than
// abort the run: losing past results is an inconvenience, losing the current
// measurement is a failure.
type Store interface {
	Append(engine.Report) error
	Recent(n int) ([]Entry, error)
	Close() error
}

// DefaultPath returns the platform's history file location.
func DefaultPath() (string, error) { return config.DefaultHistoryPath() }

// Open returns a Store backed by the file at path.
func Open(path string, limit int) (Store, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// The file may hold measurements taken at home, which is nobody elses
	// business on a shared machine.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	entries := readEntries(path)
	return &fileStore{path: path, file: f, limit: limit, entries: entries}, nil
}

// fileStore appends one JSON object per line.
//
// The format is chosen for the failure case: a truncated write damages its own
// line and nothing else, so a run interrupted mid-append costs one entry rather
// than the whole history.
type fileStore struct {
	path    string
	file    *os.File
	limit   int
	entries []Entry
}

// Append records one report.
func (s *fileStore) Append(r engine.Report) error {
	entry := EntryFromReport(r)
	s.entries = append(s.entries, entry)

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := s.file.Write(append(line, '\n')); err != nil {
		return err
	}
	if len(s.entries) > s.limit {
		return s.compact()
	}
	return nil
}

// compact rewrites the file with only the newest entries.
//
// The rewrite goes through a temporary file: overwriting in place would leave
// the history destroyed rather than merely trimmed if the process died midway.
func (s *fileStore) compact() error {
	keep := s.entries[len(s.entries)-s.limit:]

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".history-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if err := writeEntries(tmp, keep); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	// The open handle must be released before the file it points at is
	// replaced; Windows refuses the rename otherwise.
	if err := s.file.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return err
	}

	f, err := os.OpenFile(s.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	s.file = f
	s.entries = append([]Entry(nil), keep...)
	return nil
}

func writeEntries(f *os.File, entries []Entry) error {
	w := bufio.NewWriter(f)
	for _, e := range entries {
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return w.Flush()
}

// Recent returns the newest n entries, oldest first.
func (s *fileStore) Recent(n int) ([]Entry, error) {
	if n <= 0 || n > len(s.entries) {
		n = len(s.entries)
	}
	out := make([]Entry, n)
	copy(out, s.entries[len(s.entries)-n:])
	return out, nil
}

// Close releases the file.
func (s *fileStore) Close() error {
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// readEntries loads the store, skipping anything it cannot parse.
//
// A damaged line is dropped rather than raised, and an unreadable file yields
// nothing at all: the past is not worth failing the present for.
func readEntries(path string) []Entry {
	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	// A read error part way through still yields what was read: a truncated
	// history beats no history.
	return entries
}

// EntryFromReport reduces a report to the figures a trend is drawn from.
func EntryFromReport(r engine.Report) Entry {
	return Entry{
		Timestamp:    r.Timestamp,
		DownloadMbps: r.Download.Mbps,
		UploadMbps:   r.Upload.Mbps,
		LatencyMS:    r.Latency.LatencyMS,
	}
}

// Summarise derives a trend from entries, oldest first.
func Summarise(entries []Entry) Trend {
	if len(entries) > TrendWindow {
		entries = entries[len(entries)-TrendWindow:]
	}
	trend := Trend{Count: len(entries)}
	if len(entries) == 0 {
		return trend
	}

	download := series(entries, func(e Entry) float64 { return e.DownloadMbps })
	upload := series(entries, func(e Entry) float64 { return e.UploadMbps })
	lat := series(entries, func(e Entry) float64 { return e.LatencyMS })

	trend.Download = summarise(download)
	trend.Upload = summarise(upload)
	trend.Latency = summarise(lat)

	// The arrow describes the number, not the verdict. Falling latency is an
	// improvement, but deciding that is the renderer's job, not this one.
	trend.DownloadDirection = direction(download)
	trend.UploadDirection = direction(upload)
	trend.LatencyDirection = direction(lat)
	return trend
}

func series(entries []Entry, pick func(Entry) float64) []float64 {
	out := make([]float64, 0, len(entries))
	for _, e := range entries {
		out = append(out, pick(e))
	}
	return out
}

// summarise reduces a series, ignoring zeroes: a measurement that did not run
// is not a reading of zero, and averaging it in would drag every figure down.
func summarise(values []float64) Stats {
	var stats Stats
	var sum float64
	count := 0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if count == 0 || v < stats.Min {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
		sum += v
		count++
	}
	if count > 0 {
		stats.Avg = sum / float64(count)
	}
	return stats
}

// direction compares the last two usable readings.
func direction(values []float64) Direction {
	usable := make([]float64, 0, len(values))
	for _, v := range values {
		if v > 0 {
			usable = append(usable, v)
		}
	}
	if len(usable) < 2 {
		return DirectionFlat
	}
	latest := usable[len(usable)-1]
	previous := usable[len(usable)-2]

	change := (latest - previous) / previous
	switch {
	case change > flatBand:
		return DirectionUp
	case change < -flatBand:
		return DirectionDown
	default:
		return DirectionFlat
	}
}
