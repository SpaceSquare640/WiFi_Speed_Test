package output

import (
	"encoding/json"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
)

// JSON renders a report as machine-readable output: one object for a single
// pass, one object per line for continuous monitoring.
//
// Every field here is an interface others depend on. Grades and layer kinds are
// emitted as their stable identifiers, never as display text, so that changing
// the interface language cannot break a downstream parser.
type JSON struct {
	opts    Options
	encoder *json.Encoder
}

// NewJSON returns a JSON writer.
func NewJSON(o Options) *JSON {
	enc := json.NewEncoder(o.Out)
	// A single pass is read by a person as often as by a program, so it is
	// indented. A watch stream is one object per line, where an indented object
	// would break every line-oriented consumer.
	if !o.Watch {
		enc.SetIndent("", "  ")
	}
	return &JSON{opts: o, encoder: enc}
}

// The wire types below are declared separately from the engine's own types on
// purpose. They are the published contract: naming them here means a change to
// an internal struct cannot silently alter the output, and any change to the
// contract has to be made deliberately, in this file, alongside a bump of
// SchemaVersion.
type wireReport struct {
	SchemaVersion int       `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Host          wireHost  `json:"host"`

	DNS      wireDNS        `json:"dns"`
	Layers   []wireLayer    `json:"layers"`
	Download wireThroughput `json:"download"`
	Upload   wireThroughput `json:"upload"`
	Latency  wireLatency    `json:"latency"`
	Grade    string         `json:"grade"`
	Trend    *wireTrend     `json:"trend,omitempty"`

	// ResolverIsPublic warns that the second and third layers have collapsed
	// onto each other. A consumer that plots them as independent series needs
	// to know.
	ResolverIsPublic bool     `json:"resolver_is_public"`
	Errors           []string `json:"errors,omitempty"`
}

type wireHost struct {
	Name string `json:"name"`
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type wireDNS struct {
	Host      string  `json:"host"`
	OK        bool    `json:"ok"`
	ResolveMS float64 `json:"resolve_ms"`
	Error     string  `json:"error,omitempty"`
}

type wireLayer struct {
	Kind        string  `json:"kind"`
	Target      string  `json:"target"`
	UserDefined bool    `json:"user_defined"`
	OK          bool    `json:"ok"`
	LatencyMS   float64 `json:"latency_ms"`
	JitterMS    float64 `json:"jitter_ms"`
	LossPct     float64 `json:"loss_pct"`
	ProbePort   int     `json:"probe_port,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type wireThroughput struct {
	OK      bool         `json:"ok"`
	Mbps    float64      `json:"mbps"`
	Streams int          `json:"streams,omitempty"`
	Samples []wireSample `json:"samples,omitempty"`
	Error   string       `json:"error,omitempty"`
}

type wireSample struct {
	Endpoint string  `json:"endpoint"`
	Mbps     float64 `json:"mbps"`
	Error    string  `json:"error,omitempty"`
}

type wireLatency struct {
	Target    string  `json:"target,omitempty"`
	OK        bool    `json:"ok"`
	LatencyMS float64 `json:"latency_ms"`
	JitterMS  float64 `json:"jitter_ms"`
	LossPct   float64 `json:"loss_pct"`
	Error     string  `json:"error,omitempty"`
}

type wireTrend struct {
	Count             int       `json:"count"`
	Download          wireStats `json:"download"`
	Upload            wireStats `json:"upload"`
	Latency           wireStats `json:"latency"`
	DownloadDirection string    `json:"download_direction"`
	UploadDirection   string    `json:"upload_direction"`
	LatencyDirection  string    `json:"latency_direction"`
}

type wireStats struct {
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// Write emits one report.
func (j *JSON) Write(r engine.Report, t history.Trend) error {
	return j.encoder.Encode(toWire(r, t))
}

// Close flushes anything held back for the end of the run.
func (j *JSON) Close() error { return nil }

func toWire(r engine.Report, t history.Trend) wireReport {
	out := wireReport{
		SchemaVersion:    r.SchemaVersion,
		Timestamp:        r.Timestamp,
		Host:             wireHost{Name: r.Host.Name, OS: r.Host.OS, Arch: r.Host.Arch},
		Grade:            string(r.Grade),
		ResolverIsPublic: r.ResolverIsPublic,
		Errors:           r.Errors,
	}

	out.DNS = wireDNS{
		Host:      r.DNS.Host,
		OK:        r.DNS.OK,
		ResolveMS: round(r.DNS.ResolveMS),
		Error:     errString(r.DNS.Err),
	}

	for _, l := range r.Layers {
		out.Layers = append(out.Layers, wireLayer{
			Kind:        string(l.Layer.Kind),
			Target:      l.Layer.Target,
			UserDefined: l.Layer.UserDefined,
			OK:          l.Result.OK,
			LatencyMS:   round(l.Result.LatencyMS),
			JitterMS:    round(l.Result.JitterMS),
			LossPct:     round(l.Result.LossPct),
			ProbePort:   l.Result.ProbePort,
			Error:       errString(l.Result.Err),
		})
	}

	out.Download = throughputToWire(r.Download.OK, r.Download.Mbps, r.Download.Streams, r.Download.Err, r.Download.Samples)
	out.Upload = throughputToWire(r.Upload.OK, r.Upload.Mbps, r.Upload.Streams, r.Upload.Err, r.Upload.Samples)

	out.Latency = wireLatency{
		Target:    r.Latency.Target,
		OK:        r.Latency.OK,
		LatencyMS: round(r.Latency.LatencyMS),
		JitterMS:  round(r.Latency.JitterMS),
		LossPct:   round(r.Latency.LossPct),
		Error:     errString(r.Latency.Err),
	}

	// An empty history is omitted rather than emitted as zeroes, which would
	// read as a measured average of nothing.
	if t.Count > 0 {
		out.Trend = &wireTrend{
			Count:             t.Count,
			Download:          statsToWire(t.Download),
			Upload:            statsToWire(t.Upload),
			Latency:           statsToWire(t.Latency),
			DownloadDirection: string(t.DownloadDirection),
			UploadDirection:   string(t.UploadDirection),
			LatencyDirection:  string(t.LatencyDirection),
		}
	}
	return out
}

func statsToWire(s history.Stats) wireStats {
	return wireStats{Avg: round(s.Avg), Min: round(s.Min), Max: round(s.Max)}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
