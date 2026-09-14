//go:build windows

package latency

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// x/sys/windows does not wrap the performance counter, so the two calls are
// bound here directly.
var (
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procQueryPerfCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerfFrequency = kernel32.NewProc("QueryPerformanceFrequency")
)

// queryPerformanceCounter reports the current tick, or false when unavailable.
func queryPerformanceCounter() (int64, bool) {
	var value int64
	r, _, _ := procQueryPerfCounter.Call(uintptr(unsafe.Pointer(&value)))
	return value, r != 0
}

// queryPerformanceFrequency reports ticks per second.
func queryPerformanceFrequency() (int64, bool) {
	var value int64
	r, _, _ := procQueryPerfFrequency.Call(uintptr(unsafe.Pointer(&value)))
	return value, r != 0
}

// Windows resolves time.Now() to roughly one millisecond. That is coarser than
// the thing being measured: a local gateway answers in a fraction of that, so
// the first diagnostic layer — the headline of this whole tool — would report a
// flat zero. The performance counter resolves to well under a microsecond.
var qpcFrequency = func() int64 {
	freq, ok := queryPerformanceFrequency()
	if !ok {
		return 0
	}
	return freq
}()

// timer measures a single elapsed interval.
type timer struct {
	counter  int64
	fallback time.Time
}

// startTimer begins timing.
func startTimer() timer {
	if qpcFrequency == 0 {
		return timer{fallback: time.Now()}
	}
	counter, ok := queryPerformanceCounter()
	if !ok {
		return timer{fallback: time.Now()}
	}
	return timer{counter: counter}
}

// elapsed returns the time since the timer started.
func (t timer) elapsed() time.Duration {
	if !t.fallback.IsZero() {
		return time.Since(t.fallback)
	}
	now, ok := queryPerformanceCounter()
	if !ok {
		return 0
	}
	ticks := now - t.counter
	if ticks < 0 {
		return 0
	}
	// Scaled in two steps so that a long interval cannot overflow the
	// multiplication by a billion.
	seconds := ticks / qpcFrequency
	remainder := ticks % qpcFrequency
	return time.Duration(seconds)*time.Second +
		time.Duration(remainder*int64(time.Second)/qpcFrequency)
}
