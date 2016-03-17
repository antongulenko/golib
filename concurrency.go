package golib

import "time"

type Stopper struct {
	stopped   chan bool
	isStopped bool
}

func NewStopper() *Stopper {
	return &Stopper{
		stopped: make(chan bool, 1),
	}
}

func (s *Stopper) Stop() {
	s.isStopped = true
	s.stopped <- true
}

func (s *Stopper) Stopped(timeout time.Duration) bool {
	if s.IsStopped() {
		return true
	}
	select {
	case <-time.After(timeout):
		return false
	case <-s.stopped:
		s.stopped <- true
		return true
	}
}

func (s *Stopper) IsStopped() bool {
	return s.isStopped
}

// Whan receiving from this, always call Stop() afterwards!
func (s *Stopper) Wait() <-chan bool {
	return s.stopped
}
