package output

import (
	"fmt"
	"strings"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/grade"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/layers"
)

// ANSI colours. The semantics are shared with the other shells so that all
// three read as one product: download green, upload blue, latency amber,
// failure red.
const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiGreen = "\033[32m"
	ansiBlue  = "\033[34m"
	ansiAmber = "\033[33m"
	ansiRed   = "\033[31m"
)

// labelWidth is the column the values line up on, in terminal columns.
const labelWidth = 12

// Human renders a report for a person reading a terminal.
//
// The layered diagnostic block is the centrepiece. It is the one thing this tool
// does that a speed test does not, and the layout should give it that weight
// rather than flattening it into a list of numbers.
//
// Colour follows the project's semantics — download green, upload blue, latency
// amber, failure red — shared with the other shells so that all three read as
// one product. Colour is suppressed when the destination is not a terminal and
// when NO_COLOR is set.
type Human struct {
	opts Options
	tr   translator
	runs int
}

// NewHuman returns a Human writer.
func NewHuman(o Options) *Human {
	return &Human{opts: o, tr: newTranslator(o.Lang)}
}

// Write emits one report.
func (h *Human) Write(r engine.Report, t history.Trend) error {
	var b strings.Builder

	// In watch mode the reports stack up in one scrollback; a blank line
	// between them keeps the reader from mistaking one pass for another.
	if h.runs > 0 {
		b.WriteString("\n")
	}
	h.runs++

	h.writeHeader(&b, r)
	h.writeSummary(&b, r)
	h.writeLayers(&b, r)
	h.writeDNS(&b, r)
	h.writeTrend(&b, t)
	h.writeProblems(&b, r)

	_, err := fmt.Fprint(h.opts.Out, b.String())
	return err
}

// Close flushes anything held back for the end of the run.
func (h *Human) Close() error { return nil }

func (h *Human) writeHeader(b *strings.Builder, r engine.Report) {
	fmt.Fprintf(b, "%s  %s\n",
		h.paint(h.tr.t("title"), ansiBold),
		h.paint(r.Timestamp.Format("2006-01-02 15:04:05"), ansiDim))

	host := r.Host.Name
	if host == "" {
		host = "-"
	}
	fmt.Fprintf(b, "%s\n\n", h.paint(
		fmt.Sprintf("%s: %s (%s/%s)", h.tr.t("host"), host, r.Host.OS, r.Host.Arch), ansiDim))
}

func (h *Human) writeSummary(b *strings.Builder, r engine.Report) {
	h.writeField(b, h.tr.t("download"), h.rate(r.Download.OK, r.Download.Mbps, ansiGreen))
	h.writeField(b, h.tr.t("upload"), h.rate(r.Upload.OK, r.Upload.Mbps, ansiBlue))
	if r.Latency.OK {
		h.writeField(b, h.tr.t("latency"), h.paint(fmt.Sprintf("%.1f ms", r.Latency.LatencyMS), ansiAmber))
	}
	h.writeField(b, h.tr.t("grade"), h.gradeText(r.Grade))
	b.WriteString("\n")
}

// writeField prints one label and value, aligned by display width so that the
// Chinese and English interfaces line up alike.
func (h *Human) writeField(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "  %s %s\n", padRight(label+":", labelWidth), value)
}

// rate renders a throughput figure, or says plainly that it was never measured.
// Printing 0.0 Mbps for an absent measurement would claim a result the run does
// not have.
func (h *Human) rate(ok bool, mbps float64, colour string) string {
	if !ok {
		return h.paint(h.tr.t("notMeasured"), ansiDim)
	}
	return h.paint(fmt.Sprintf("%.2f Mbps", mbps), colour)
}

func (h *Human) gradeText(g grade.Grade) string {
	text := h.tr.t("grade_" + string(g))
	switch g {
	case grade.GradeExcellent, grade.GradeVeryGood:
		return h.paint(text, ansiGreen)
	case grade.GradeGood, grade.GradeFair:
		return h.paint(text, ansiAmber)
	case grade.GradeSlow:
		return h.paint(text, ansiRed)
	default:
		return h.paint(text, ansiDim)
	}
}

// layerRow is one line of the diagnostic table, kept in both its plain and its
// painted form: the widths are measured from the plain text, because an escape
// sequence occupies bytes but no columns.
type layerRow struct {
	plain   [6]string
	painted [6]string
}

// writeLayers renders the centrepiece.
//
// The columns are measured and padded here rather than handed to tabwriter,
// which counts runes: a Chinese layer name would be padded to half the width it
// actually occupies, and the whole block would lean.
func (h *Human) writeLayers(b *strings.Builder, r engine.Report) {
	if len(r.Layers) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s\n", h.paint(h.tr.t("diagnostics"), ansiBold))

	rows := []layerRow{h.headerRow()}
	for _, l := range r.Layers {
		rows = append(rows, h.layerRow(l))
	}

	var widths [6]int
	for _, row := range rows {
		for i, cell := range row.plain {
			if w := displayWidth(cell); w > widths[i] {
				widths[i] = w
			}
		}
	}

	for _, row := range rows {
		b.WriteString("  ")
		for i := range row.plain {
			b.WriteString(row.painted[i])
			if i < len(row.plain)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-displayWidth(row.plain[i])+2))
			}
		}
		b.WriteString("\n")
	}

	if r.ResolverIsPublic {
		fmt.Fprintf(b, "  %s\n", h.paint("! "+h.tr.t("resolverPublic"), ansiAmber))
	}
	b.WriteString("\n")
}

func (h *Human) headerRow() layerRow {
	var row layerRow
	for i, key := range []string{"layer", "target", "port", "latency", "jitter", "loss"} {
		row.plain[i] = strings.ToUpper(h.tr.t(key))
		row.painted[i] = h.paint(row.plain[i], ansiDim)
	}
	return row
}

func (h *Human) layerRow(l engine.LayerResult) layerRow {
	var row layerRow

	name := h.layerName(l.Layer.Kind)
	if l.Layer.UserDefined {
		name += " (" + h.tr.t("custom") + ")"
	}
	port := "-"
	if l.Result.ProbePort > 0 {
		port = fmt.Sprintf("%d", l.Result.ProbePort)
	}
	row.plain = [6]string{name, l.Layer.Target, port, "", "", ""}

	if l.Result.OK {
		row.plain[3] = fmt.Sprintf("%.2f ms", l.Result.LatencyMS)
		row.plain[4] = fmt.Sprintf("%.2f ms", l.Result.JitterMS)
		row.plain[5] = fmt.Sprintf("%.0f %%", l.Result.LossPct)
	} else {
		// A dead layer is the most informative row this tool can print. It is
		// stated, not blanked.
		row.plain[3] = h.tr.t("unreachable")
	}

	row.painted = row.plain
	if l.Result.OK {
		row.painted[3] = h.paint(row.plain[3], ansiAmber)
		if l.Result.LossPct > 0 {
			row.painted[5] = h.paint(row.plain[5], ansiRed)
		}
	} else {
		row.painted[3] = h.paint(row.plain[3], ansiRed)
	}
	return row
}

func (h *Human) layerName(k layers.Kind) string {
	switch k {
	case layers.KindGateway:
		return h.tr.t("gateway")
	case layers.KindRegionalEgress:
		return h.tr.t("regional")
	case layers.KindInternational:
		return h.tr.t("international")
	default:
		return string(k)
	}
}

func (h *Human) writeDNS(b *strings.Builder, r engine.Report) {
	if r.DNS.Host == "" {
		return
	}
	if !r.DNS.OK {
		h.writeField(b, h.tr.t("dns"), h.paint(h.tr.t("unreachable"), ansiRed))
		b.WriteString("\n")
		return
	}
	h.writeField(b, h.tr.t("dns"), fmt.Sprintf("%.2f ms %s",
		r.DNS.ResolveMS, h.paint("("+r.DNS.Host+")", ansiDim)))
	b.WriteString("\n")
}

func (h *Human) writeTrend(b *strings.Builder, t history.Trend) {
	// One run is not a trend. Printing an average of a single measurement
	// dresses one sample as a pattern.
	if t.Count < 2 {
		return
	}
	// A heading with no lines beneath it claims a trend that was never
	// measured.
	if t.Download.Avg == 0 && t.Upload.Avg == 0 && t.Latency.Avg == 0 {
		return
	}

	fmt.Fprintf(b, "  %s %s\n",
		h.paint(h.tr.t("trend"), ansiBold),
		h.paint(fmt.Sprintf("(%d %s)", t.Count, h.tr.t("runs")), ansiDim))

	h.writeTrendLine(b, h.tr.t("download"), t.Download, t.DownloadDirection, "Mbps", ansiGreen)
	h.writeTrendLine(b, h.tr.t("upload"), t.Upload, t.UploadDirection, "Mbps", ansiBlue)
	h.writeTrendLine(b, h.tr.t("latency"), t.Latency, t.LatencyDirection, "ms", ansiAmber)
	b.WriteString("\n")
}

func (h *Human) writeTrendLine(b *strings.Builder, label string, s history.Stats, d history.Direction, unit, colour string) {
	if s.Avg == 0 {
		return
	}
	fmt.Fprintf(b, "  %s %s %s   %s %.2f  %s %.2f  %s %.2f\n",
		padRight(label+":", labelWidth),
		h.paint(padRight(fmt.Sprintf("%.2f %s", s.Avg, unit), 12), colour),
		arrow(d),
		h.tr.t("min"), s.Min,
		h.tr.t("avg"), s.Avg,
		h.tr.t("max"), s.Max)
}

// arrow shows which way the latest reading moved. It describes the number only;
// whether a rise is good depends on what is being measured.
func arrow(d history.Direction) string {
	switch d {
	case history.DirectionUp:
		return "^"
	case history.DirectionDown:
		return "v"
	default:
		return "-"
	}
}

func (h *Human) writeProblems(b *strings.Builder, r engine.Report) {
	if len(r.Errors) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s\n", h.paint(h.tr.t("problems"), ansiBold))
	for _, e := range r.Errors {
		fmt.Fprintf(b, "  %s\n", h.paint("- "+e, ansiRed))
	}
	b.WriteString("\n")
}

// paint applies a colour when colour is enabled.
func (h *Human) paint(text, colour string) string {
	if !h.opts.Color || colour == "" {
		return text
	}
	return colour + text + ansiReset
}
