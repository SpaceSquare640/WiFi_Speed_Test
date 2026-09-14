package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/latency"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

func reportWith(download, upload, lat float64) engine.Report {
	return engine.Report{
		Timestamp: time.Now(),
		Download:  throughput.Result{OK: true, Mbps: download},
		Upload:    throughput.Result{OK: true, Mbps: upload},
		Latency:   latency.Result{OK: true, LatencyMS: lat},
	}
}

func TestAppendAndRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for i := 1; i <= 3; i++ {
		if err := store.Append(reportWith(float64(i*10), float64(i), float64(i))); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := store.Recent(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// Oldest first, so the newest of the two is last.
	if entries[1].DownloadMbps != 30 {
		t.Errorf("newest entry = %v, want 30", entries[1].DownloadMbps)
	}
}

// Reopening must see what the previous run wrote; a history that forgets on
// restart is not a history.
func TestPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(reportWith(50, 5, 20)); err != nil {
		t.Fatal(err)
	}
	store.Close()

	reopened, err := Open(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	entries, _ := reopened.Recent(0)
	if len(entries) != 1 || entries[0].DownloadMbps != 50 {
		t.Errorf("entries = %+v, want the previously written one", entries)
	}
}

func TestLimitTrimsOldestEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store, err := Open(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for i := 1; i <= 6; i++ {
		if err := store.Append(reportWith(float64(i), 0, 0)); err != nil {
			t.Fatal(err)
		}
	}

	entries, _ := store.Recent(0)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want the limit of 3", len(entries))
	}
	if entries[0].DownloadMbps != 4 || entries[2].DownloadMbps != 6 {
		t.Errorf("kept the wrong entries: %+v", entries)
	}

	// The trim must have reached the file, not just the in-memory slice.
	reopened, err := Open(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if entries, _ := reopened.Recent(0); len(entries) != 3 {
		t.Errorf("file holds %d entries after compaction, want 3", len(entries))
	}
}

// Losing past results is an inconvenience; losing the current measurement is a
// failure. A damaged line must not take the run down with it.
func TestCorruptLinesAreSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	body := "{\"download_mbps\":10}\nnot json at all\n{\"download_mbps\":20}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path, 0)
	if err != nil {
		t.Fatalf("a damaged history must still open: %v", err)
	}
	defer store.Close()

	entries, _ := store.Recent(0)
	if len(entries) != 2 {
		t.Errorf("got %d entries, want the 2 readable ones", len(entries))
	}
}

func TestSummariseStats(t *testing.T) {
	entries := []Entry{
		{DownloadMbps: 100, UploadMbps: 10, LatencyMS: 20},
		{DownloadMbps: 200, UploadMbps: 20, LatencyMS: 30},
		{DownloadMbps: 300, UploadMbps: 30, LatencyMS: 40},
	}
	trend := Summarise(entries)

	if trend.Count != 3 {
		t.Errorf("Count = %d, want 3", trend.Count)
	}
	if trend.Download.Avg != 200 || trend.Download.Min != 100 || trend.Download.Max != 300 {
		t.Errorf("Download = %+v", trend.Download)
	}
	if trend.DownloadDirection != DirectionUp {
		t.Errorf("DownloadDirection = %q, want up", trend.DownloadDirection)
	}
}

// A measurement that did not run is not a reading of zero. Averaging it in
// would drag every figure down and invent a decline that never happened.
func TestUnmeasuredValuesAreIgnored(t *testing.T) {
	entries := []Entry{
		{DownloadMbps: 100},
		{DownloadMbps: 0}, // throughput was skipped on this pass
		{DownloadMbps: 100},
	}
	trend := Summarise(entries)

	if trend.Download.Avg != 100 {
		t.Errorf("Avg = %v, want 100", trend.Download.Avg)
	}
	if trend.Download.Min != 100 {
		t.Errorf("Min = %v, want 100", trend.Download.Min)
	}
	if trend.DownloadDirection != DirectionFlat {
		t.Errorf("Direction = %q, want flat", trend.DownloadDirection)
	}
}

// Without a band every run would report a direction, and an arrow that always
// points somewhere tells the reader nothing.
func TestSmallChangesReadAsFlat(t *testing.T) {
	cases := map[string]struct {
		previous, latest float64
		want             Direction
	}{
		"within the band":  {100, 102, DirectionFlat},
		"clearly up":       {100, 130, DirectionUp},
		"clearly down":     {100, 70, DirectionDown},
		"exactly the band": {100, 105, DirectionFlat},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			trend := Summarise([]Entry{{DownloadMbps: c.previous}, {DownloadMbps: c.latest}})
			if trend.DownloadDirection != c.want {
				t.Errorf("direction = %q, want %q", trend.DownloadDirection, c.want)
			}
		})
	}
}

func TestSummariseWindow(t *testing.T) {
	entries := make([]Entry, TrendWindow+10)
	for i := range entries {
		entries[i] = Entry{DownloadMbps: float64(i + 1)}
	}
	trend := Summarise(entries)
	if trend.Count != TrendWindow {
		t.Errorf("Count = %d, want the window of %d", trend.Count, TrendWindow)
	}
}

func TestSummariseEmpty(t *testing.T) {
	trend := Summarise(nil)
	if trend.Count != 0 || trend.Download.Avg != 0 {
		t.Errorf("an empty history must summarise to nothing: %+v", trend)
	}
}
