//go:build !windows

package latency

import "time"

// timer measures a single elapsed interval.
type timer struct{ start time.Time }

// startTimer begins timing.
func startTimer() timer { return timer{start: time.Now()} }

// elapsed returns the time since the timer started.
func (t timer) elapsed() time.Duration { return time.Since(t.start) }
