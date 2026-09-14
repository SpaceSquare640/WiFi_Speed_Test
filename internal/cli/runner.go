package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/SpaceSquare640/WiFi_Speed_Test/core/engine"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/history"
	"github.com/SpaceSquare640/WiFi_Speed_Test/core/notify"
	"github.com/SpaceSquare640/WiFi_Speed_Test/internal/cli/output"
)

// Runner drives the engine in either mode and fans each report out to the
// writer, the history store and the notifier.
type Runner struct {
	engine   *engine.Engine
	store    history.Store
	notifier notify.Notifier
	writer   output.Writer
	stderr   io.Writer

	interval time.Duration
	count    int
}

// RunOnce performs a single pass.
func (r *Runner) RunOnce(ctx context.Context) (ExitCode, error) {
	report, err := r.engine.Run(ctx)
	r.deliver(ctx, report)

	if ctx.Err() != nil {
		return ExitInterrupted, nil
	}
	code := classify(report, err)
	if code == ExitNoNetwork {
		return code, err
	}
	return code, nil
}

// RunWatch repeats passes until the context ends or the configured count is
// reached.
//
// A failed cycle must not end the loop: monitoring exists precisely to observe
// a connection that comes and goes. Cancellation has to take effect at once
// rather than after the current interval elapses.
func (r *Runner) RunWatch(ctx context.Context) (ExitCode, error) {
	interval := r.interval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	// The status of a watch is the status of its last completed pass. A run
	// that ends while the network happens to be down should say so.
	last := ExitOK
	passes := 0

	for {
		report, err := r.engine.Run(ctx)
		if ctx.Err() != nil {
			return ExitInterrupted, nil
		}
		r.deliver(ctx, report)
		last = classify(report, err)
		passes++

		if r.count > 0 && passes >= r.count {
			return last, nil
		}

		select {
		case <-ctx.Done():
			return ExitInterrupted, nil
		case <-time.After(interval):
		}
	}
}

// deliver renders the report and fans it out to history and the notifier.
//
// None of these are allowed to fail the run. The measurement has already
// happened; a webhook that is down or a disk that is full does not make it
// untrue, so the failures are reported on stderr and the pass stands.
func (r *Runner) deliver(ctx context.Context, report engine.Report) {
	trend := r.trend(report)

	if err := r.writer.Write(report, trend); err != nil {
		r.warn("output: %v", err)
	}
	if r.store != nil {
		if err := r.store.Append(report); err != nil {
			r.warn("history: %v", err)
		}
	}
	if r.notifier != nil {
		if err := r.notifier.Notify(ctx, report, trend); err != nil && !errors.Is(err, context.Canceled) {
			r.warn("notify: %v", err)
		}
	}
}

// trend summarises the stored history, including the pass just taken.
func (r *Runner) trend(report engine.Report) history.Trend {
	if r.store == nil {
		return history.Trend{}
	}
	entries, err := r.store.Recent(history.TrendWindow)
	if err != nil {
		r.warn("history: %v", err)
		return history.Trend{}
	}
	// The fresh report is not in the store yet; appending it here is what makes
	// the trend describe the run the user is looking at.
	entries = append(entries, history.EntryFromReport(report))
	return history.Summarise(entries)
}

func (r *Runner) warn(format string, args ...any) {
	if r.stderr == nil {
		return
	}
	fmt.Fprintf(r.stderr, format+"\n", args...)
}
