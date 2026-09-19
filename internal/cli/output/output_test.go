package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/dns"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/grade"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/latency"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/layers"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/throughput"
)

func sampleReport() engine.Report {
	return engine.Report{
		SchemaVersion: engine.SchemaVersion,
		Timestamp:     time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC),
		Host:          engine.Host{Name: "testhost", OS: "linux", Arch: "arm64"},
		DNS:           dns.Result{Host: "example.com", OK: true, ResolveMS: 4.1234567},
		Layers: []engine.LayerResult{
			{
				Layer:  layers.Layer{Kind: layers.KindGateway, Target: "192.168.1.1"},
				Result: latency.Result{OK: true, LatencyMS: 1.5, JitterMS: 0.2, ProbePort: 80},
			},
			{
				Layer:  layers.Layer{Kind: layers.KindInternational, Target: "1.1.1.1"},
				Result: latency.Result{LossPct: 100, ProbePort: 53},
			},
		},
		Download: throughput.Result{OK: true, Mbps: 123.456},
		Grade:    grade.GradeGood,
	}
}

func TestJSONShape(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSON(Options{Out: &buf})
	if err := w.Write(sampleReport(), history.Trend{}); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if got["schema_version"].(float64) != float64(engine.SchemaVersion) {
		t.Errorf("schema_version = %v", got["schema_version"])
	}
	// The grade is emitted as its stable identifier, never as display text:
	// changing the interface language must not break a parser.
	if got["grade"].(string) != string(grade.GradeGood) {
		t.Errorf("grade = %v, want the identifier %q", got["grade"], grade.GradeGood)
	}
	layerList := got["layers"].([]any)
	if kind := layerList[0].(map[string]any)["kind"].(string); kind != string(layers.KindGateway) {
		t.Errorf("layer kind = %q, want the identifier", kind)
	}
	// An empty history is omitted rather than emitted as zeroes, which would
	// read as a measured average of nothing.
	if _, present := got["trend"]; present {
		t.Error("trend emitted for an empty history")
	}
}

func TestJSONRoundsNoise(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSON(Options{Out: &buf})
	if err := w.Write(sampleReport(), history.Trend{}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(buf.Bytes(), &got)

	if resolve := got["dns"].(map[string]any)["resolve_ms"].(float64); resolve != 4.123 {
		t.Errorf("resolve_ms = %v, want 4.123", resolve)
	}
}

// Watch mode is consumed line by line, so an indented object would break every
// line-oriented reader.
func TestJSONWatchIsOnePerLine(t *testing.T) {
	var buf bytes.Buffer
	w := NewJSON(Options{Out: &buf, Watch: true})
	_ = w.Write(sampleReport(), history.Trend{})
	_ = w.Write(sampleReport(), history.Trend{})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want one object per pass", len(lines))
	}
	for i, line := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line %d is not a complete object: %v", i, err)
		}
	}
}

func TestHumanStatesWhatWasNotMeasured(t *testing.T) {
	var buf bytes.Buffer
	r := sampleReport()
	r.Upload = throughput.Result{} // never ran

	w := NewHuman(Options{Out: &buf})
	if err := w.Write(r, history.Trend{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Printing 0.00 Mbps would claim a result the run does not have.
	if !strings.Contains(out, "not measured") {
		t.Errorf("an unmeasured upload must say so:\n%s", out)
	}
	if strings.Contains(out, "0.00 Mbps") {
		t.Errorf("an unmeasured upload must not be rendered as a rate:\n%s", out)
	}
	// A dead layer is the most informative row this tool prints.
	if !strings.Contains(out, "unreachable") {
		t.Errorf("an unreachable layer must be stated:\n%s", out)
	}
}

func TestHumanColourIsOptional(t *testing.T) {
	var plain, painted bytes.Buffer
	_ = NewHuman(Options{Out: &plain}).Write(sampleReport(), history.Trend{})
	_ = NewHuman(Options{Out: &painted, Color: true}).Write(sampleReport(), history.Trend{})

	if strings.Contains(plain.String(), "\033[") {
		t.Error("escape sequences emitted with colour disabled")
	}
	if !strings.Contains(painted.String(), "\033[") {
		t.Error("no escape sequences emitted with colour enabled")
	}
}

// The Chinese interface must line up like the English one. Padding by rune
// count leaves every column leaning.
func TestHumanAlignsChineseColumns(t *testing.T) {
	var buf bytes.Buffer
	w := NewHuman(Options{Out: &buf, Lang: LangChinese})
	if err := w.Write(sampleReport(), history.Trend{}); err != nil {
		t.Fatal(err)
	}

	var targets []int
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "192.168.1.1") || strings.Contains(line, "1.1.1.1") {
			targets = append(targets, displayWidth(line[:strings.Index(line, "1")]))
		}
	}
	if len(targets) != 2 {
		t.Fatalf("expected two layer rows, found %d", len(targets))
	}
	if targets[0] != targets[1] {
		t.Errorf("layer targets start at columns %d and %d; they must align", targets[0], targets[1])
	}
}

func TestDisplayWidth(t *testing.T) {
	cases := map[string]int{
		"":            0,
		"abc":         3,
		"下載":          4,
		"本地網關":        8,
		"下載: 100Mbps": 13, // 4 + 1 + 1 + 7
	}
	for input, want := range cases {
		if got := displayWidth(input); got != want {
			t.Errorf("displayWidth(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestPadRight(t *testing.T) {
	if got := padRight("下載:", 10); displayWidth(got) != 10 {
		t.Errorf("padRight produced width %d, want 10", displayWidth(got))
	}
	// A string already wider than the target is never truncated: losing a
	// character to alignment would be worse than a ragged column.
	if got := padRight("a very long label", 4); got != "a very long label" {
		t.Errorf("padRight truncated: %q", got)
	}
}

// One run is not a trend, and a heading with nothing under it claims one.
func TestHumanSuppressesEmptyTrend(t *testing.T) {
	var buf bytes.Buffer
	w := NewHuman(Options{Out: &buf})
	_ = w.Write(sampleReport(), history.Trend{Count: 4}) // counted, but nothing measured

	if strings.Contains(buf.String(), "Trend") {
		t.Errorf("a trend with no measured values must be omitted:\n%s", buf.String())
	}
}

func TestTranslatorFallsBackToEnglish(t *testing.T) {
	if got := newTranslator("fr").t("download"); got != "Download" {
		t.Errorf("unknown language = %q, want the English text", got)
	}
	// Users type what they know; zh, zh-tw and zh_TW all mean one thing.
	for _, tag := range []string{"zh", "zh-TW", "zh_tw", "ZH-tw"} {
		if got := newTranslator(tag).t("download"); got != "下載" {
			t.Errorf("newTranslator(%q) = %q, want the Chinese text", tag, got)
		}
	}
}

// A platform that never had a resolver configuration has not failed at
// measuring one. These two rows exist so Android does not read as a broken
// host, and they only appear there — which is a platform this suite cannot run
// on, so the rendering is pinned here instead.
func TestHumanSeparatesUnsupportedFromFailed(t *testing.T) {
	var buf bytes.Buffer
	r := sampleReport()
	r.RegionalUnsupported = true
	r.DNS = dns.Result{Host: "example.com", Err: dns.ErrUnsupported}

	if err := NewHuman(Options{Out: &buf}).Write(r, history.Trend{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "not supported on this platform") {
		t.Errorf("an unsupported measurement must say so rather than vanish:\n%s", out)
	}
	// "unreachable" is a fault. Reporting one where none exists would send the
	// user hunting a problem with their network.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "DNS") && strings.Contains(line, "unreachable") {
			t.Errorf("an unsupported platform must not be reported as unreachable:\n%s", line)
		}
	}
	if !strings.Contains(out, "never attempted") {
		t.Errorf("the missing regional row must explain itself:\n%s", out)
	}
	// It is a fact about the platform, not a warning, so it must not be shouted.
	var painted bytes.Buffer
	_ = NewHuman(Options{Out: &painted, Color: true}).Write(r, history.Trend{})
	if strings.Contains(painted.String(), ansiRed+"- no regional egress row") {
		t.Error("an unsupported layer must not be coloured as a failure")
	}
}

// The reasons individual endpoints gave were collected all along and only the
// machine format printed them, which left "every endpoint failed" as the whole
// of what a person was told.
func TestHumanNamesWhyEachEndpointFailed(t *testing.T) {
	var buf bytes.Buffer
	r := sampleReport()
	r.Download = throughput.Result{
		Direction: throughput.DirectionDownload,
		Samples: []throughput.Sample{
			{Endpoint: "https://example.com/down", Err: errors.New("the endpoint answered 429 Too Many Requests")},
		},
		Err: throughput.ErrAllEndpointsFailed,
	}
	r.Errors = []string{"download: " + throughput.ErrAllEndpointsFailed.Error()}

	if err := NewHuman(Options{Out: &buf}).Write(r, history.Trend{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "429 Too Many Requests") {
		t.Errorf("the reason must reach the person, not only the JSON:\n%s", out)
	}
	if !strings.Contains(out, "https://example.com/down") {
		t.Errorf("the reason must say which endpoint gave it:\n%s", out)
	}
}
