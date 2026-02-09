package timer

import (
	"time"
)

type Timer struct {
	internalTimer *time.Timer
	duration      time.Duration
	C             chan bool
}

func New(duration time.Duration) *Timer {
	t := &Timer{
		internalTimer: time.NewTimer(0),
		duration:      duration,
		C:             make(chan bool, 1),
	}
	t.internalTimer.Stop()
	select {
	case <-t.internalTimer.C:
	default:
	}
	return t
}

func Stop(t *Timer) {
	if !t.internalTimer.Stop() {
		select {
		case <-t.internalTimer.C:
		default:
		}
	}
}

func Start(t *Timer) {
	Stop(t)
	t.internalTimer.Reset(t.duration)

	go func() {
		select {
		case <-t.internalTimer.C:
			t.C <- true
		}
	}()
}
