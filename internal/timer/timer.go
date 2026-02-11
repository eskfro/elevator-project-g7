package timer

import (
	"time"
)

type Timer struct {
	internal *time.Timer
	duration time.Duration
	C        chan struct{}
	stop     chan struct{}
}

func New(duration time.Duration) *Timer {
	t := &Timer{
		internal: time.NewTimer(duration),
		duration: duration,
		C:        make(chan struct{}, 1),
		stop:     make(chan struct{}),
	}
	t.internal.Stop()

	go t.loop()
	return t
}

func (t *Timer) loop() {
	for {
		select {
		case <-t.internal.C:
			select {
			case t.C <- struct{}{}:
			default:
			}
		case <-t.stop:
			return
		}
	}
}

func (t *Timer) Start() {
	if !t.internal.Stop() {
		select {
		case <-t.internal.C:
		default:
		}
	}
	t.internal.Reset(t.duration)
}

func (t *Timer) Stop() {
	if !t.internal.Stop() {
		select {
		case <-t.internal.C:
		default:
		}
	}
}

func (t *Timer) Close() {
	close(t.stop)
	t.Stop()
}
