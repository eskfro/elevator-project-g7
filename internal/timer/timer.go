package timer

import (
	"time"
)

type Timer struct {
	internal *time.Timer
	duration time.Duration
	C        chan bool
	stop     chan struct{}
}

func New(duration time.Duration) *Timer {
	t := &Timer{
		internal: time.NewTimer(0),
		duration: duration,
		C:        make(chan bool, 1),
		stop:     make(chan struct{}),
	}
	t.internal.Stop()

	go func() {
		for {
			select {
			case <-t.internal.C:
				select {
				case t.C <- true:
				default:
				}
			case <-t.stop:
				return //exit point
			}
		}
	}()

	return t

}

func Stop(t *Timer) {
	if !t.internal.Stop() {
		select {
		case <-t.internal.C:
		default:
		}
	}
}

func Start(t *Timer) {
	if !t.internal.Stop() {
		select {
		case <-t.internal.C:
		default:
		}
	}
	t.internal.Reset(t.duration)
}

func Set(t *Timer, _duration time.Duration) {
	t.duration = _duration
	Start(t)
}
