//go:build windows

package dns

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows resolves time.Now() to roughly one millisecond, which is coarser than
// a cached lookup takes. The performance counter resolves far finer. The same
// reasoning, and the same binding, appear in core/latency.
var (
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procQueryPerfCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerfFrequency = kernel32.NewProc("QueryPerformanceFrequency")

	qpcFrequency = func() int64 {
		var value int64
		r, _, _ := procQueryPerfFrequency.Call(uintptr(unsafe.Pointer(&value)))
		if r == 0 {
			return 0
		}
		return value
	}()
)

type timer struct {
	counter  int64
	fallback time.Time
}

func startTimer() timer {
	if qpcFrequency == 0 {
		return timer{fallback: time.Now()}
	}
	var value int64
	if r, _, _ := procQueryPerfCounter.Call(uintptr(unsafe.Pointer(&value))); r == 0 {
		return timer{fallback: time.Now()}
	}
	return timer{counter: value}
}

func (t timer) elapsed() time.Duration {
	if !t.fallback.IsZero() {
		return time.Since(t.fallback)
	}
	var value int64
	if r, _, _ := procQueryPerfCounter.Call(uintptr(unsafe.Pointer(&value))); r == 0 {
		return 0
	}
	ticks := value - t.counter
	if ticks < 0 {
		return 0
	}
	return time.Duration(ticks/qpcFrequency)*time.Second +
		time.Duration(ticks%qpcFrequency*int64(time.Second)/qpcFrequency)
}
