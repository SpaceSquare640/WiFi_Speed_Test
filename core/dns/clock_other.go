//go:build !windows

package dns

import "time"

type timer struct{ start time.Time }

func startTimer() timer { return timer{start: time.Now()} }

func (t timer) elapsed() time.Duration { return time.Since(t.start) }
