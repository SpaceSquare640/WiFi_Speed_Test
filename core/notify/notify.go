// Package notify pushes a finished report to a webhook the user supplies.
//
// The project operates no receiver of its own and ships no default destination.
// The feature stays inert until a URL is configured, and that URL is a
// credential: it lives in local configuration or the environment, never in
// source and never in version control.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/grade"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
)

// Notifier delivers a report somewhere the user chose.
type Notifier interface {
	Notify(ctx context.Context, r engine.Report, t history.Trend) error
}

// Format identifies a webhook payload dialect.
type Format string

const (
	FormatDiscord Format = "discord"
)

// Delivery settings.
const (
	deliveryTimeout = 10 * time.Second
	retryBackoff    = time.Second
)

// Colours used in the Discord embed, matching the grade bands.
const (
	colourGood    = 0x2ecc71
	colourFair    = 0xf1c40f
	colourPoor    = 0xe74c3c
	colourUnknown = 0x95a5a6
)

// Options configures a webhook notifier.
type Options struct {
	URL     string
	Format  Format
	Retries int
}

// New returns a Notifier for the given options, or nil when URL is empty.
// A nil Notifier is a valid no-op and callers need not special-case it.
func New(o Options) Notifier {
	if strings.TrimSpace(o.URL) == "" {
		return nil
	}
	if o.Format == "" {
		o.Format = FormatDiscord
	}
	if o.Retries < 1 {
		o.Retries = 1
	}
	return &webhook{opts: o, client: &http.Client{Timeout: deliveryTimeout}}
}

type webhook struct {
	opts   Options
	client *http.Client
}

// Notify posts the report to the configured webhook.
func (w *webhook) Notify(ctx context.Context, r engine.Report, t history.Trend) error {
	body, err := json.Marshal(discordPayload(r, t))
	if err != nil {
		return err
	}

	var lastErr error
	delay := retryBackoff
	for attempt := 0; attempt < w.opts.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
		if err := w.post(ctx, body); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func (w *webhook) post(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.opts.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		// The URL is a credential and must never reach a log or a terminal, so
		// the transport error is reported without it.
		return fmt.Errorf("webhook delivery failed: %w", redact(err, w.opts.URL))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook delivery failed with status %d", resp.StatusCode)
	}
	return nil
}

// redact removes the webhook URL from an error message.
func redact(err error, url string) error {
	if err == nil || url == "" {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), url, "<webhook>")
	return fmt.Errorf("%s", msg)
}

// discordEmbed is the subset of the Discord webhook schema this tool uses.
type discordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordMessage struct {
	Embeds []discordEmbed `json:"embeds"`
}

// discordPayload renders the report as a Discord embed.
//
// The layered diagnostics are the body rather than a footnote: someone reading
// this in a chat window wants to know which segment broke, which is the one
// thing a plain speed figure cannot tell them.
func discordPayload(r engine.Report, t history.Trend) discordMessage {
	embed := discordEmbed{
		Title:     "WiFi Speed Test",
		Color:     gradeColour(r.Grade),
		Timestamp: r.Timestamp.UTC().Format(time.RFC3339),
	}
	if r.Host.Name != "" {
		embed.Description = fmt.Sprintf("%s (%s/%s)", r.Host.Name, r.Host.OS, r.Host.Arch)
	}

	embed.Fields = append(embed.Fields,
		discordEmbedField{Name: "Download", Value: rateText(r.Download.OK, r.Download.Mbps), Inline: true},
		discordEmbedField{Name: "Upload", Value: rateText(r.Upload.OK, r.Upload.Mbps), Inline: true},
		discordEmbedField{Name: "Grade", Value: string(r.Grade), Inline: true},
	)

	if len(r.Layers) > 0 {
		var b strings.Builder
		for _, l := range r.Layers {
			if !l.Result.OK {
				fmt.Fprintf(&b, "%s (%s): unreachable\n", l.Layer.Kind, l.Layer.Target)
				continue
			}
			fmt.Fprintf(&b, "%s (%s): %.2f ms, jitter %.2f ms, loss %.0f%%\n",
				l.Layer.Kind, l.Layer.Target, l.Result.LatencyMS, l.Result.JitterMS, l.Result.LossPct)
		}
		if r.ResolverIsPublic {
			b.WriteString("note: the system resolver is public, so layers 2 and 3 share a path\n")
		}
		embed.Fields = append(embed.Fields, discordEmbedField{Name: "Layered diagnostics", Value: b.String()})
	}

	if t.Count > 1 {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name: "Trend",
			Value: fmt.Sprintf("%d runs - download avg %.2f Mbps (%s), latency avg %.2f ms (%s)",
				t.Count, t.Download.Avg, t.DownloadDirection, t.Latency.Avg, t.LatencyDirection),
		})
	}

	if len(r.Errors) > 0 {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name:  "Problems",
			Value: strings.Join(r.Errors, "\n"),
		})
	}
	return discordMessage{Embeds: []discordEmbed{embed}}
}

func rateText(ok bool, mbps float64) string {
	if !ok {
		return "not measured"
	}
	return fmt.Sprintf("%.2f Mbps", mbps)
}

func gradeColour(g grade.Grade) int {
	switch g {
	case grade.GradeExcellent, grade.GradeVeryGood:
		return colourGood
	case grade.GradeGood, grade.GradeFair:
		return colourFair
	case grade.GradeSlow:
		return colourPoor
	default:
		return colourUnknown
	}
}
