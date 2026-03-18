package timer

import (
	"sync"
	"time"
)

/*
	Made our own timer-module to make the timers more intuitive to use
	without compromising safety
*/

type Timer struct {
	internal *time.Timer
	duration time.Duration
	C        chan struct{}
	stop     chan struct{}

	mu        sync.Mutex
	isRunning bool
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
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.internal.Stop() {
		select {
		case <-t.internal.C:
		default:
		}
	}

	t.internal.Reset(t.duration)
	t.isRunning = true
}

func (t *Timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.internal.Stop() {
		select {
		case <-t.internal.C:
		default:
		}
	}
	t.isRunning = false
}

func (t *Timer) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.isRunning
}

func (t *Timer) Close() {
	close(t.stop)
	t.Stop()
}
