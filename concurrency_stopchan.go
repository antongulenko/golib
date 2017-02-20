package golib

import (
	"reflect"
	"sync"
	"time"
)

type stopChan struct {
	cond     sync.Cond
	stopped  bool
	err      error
	waitChan chan error
}

// The nil-value of StopChan (e.g. StopChan{}) mostly acts like an already-stopped StopChan with a nil error.
// The only exception is the WaitForAny() function, which will ignore uninitialized StopChans.
type StopChan struct {
	*stopChan
}

func NewStopChan() StopChan {
	return StopChan{
		stopChan: &stopChan{
			cond: sync.Cond{
				L: new(sync.Mutex),
			},
		},
	}
}

func NewStoppedChan(err error) StopChan {
	res := NewStopChan()
	res.StopErr(err)
	return res
}

func (s *stopChan) StopErrFunc(perform func() error) {
	if s == nil {
		return
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	if s.stopped {
		return
	}
	if perform != nil {
		s.err = perform()
	}
	s.stopped = true
	s.cond.Broadcast()
}

func (s *stopChan) StopFunc(perform func()) {
	s.StopErrFunc(func() error {
		if perform != nil {
			perform()
		}
		return nil
	})
}

func (s *stopChan) StopErr(err error) {
	s.StopErrFunc(func() error {
		return err
	})
}

func (s *stopChan) Stop() {
	s.StopErrFunc(nil)
}

func (s *stopChan) Stopped() bool {
	if s == nil {
		return true
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	return s.stopped
}

func (s *stopChan) Err() error {
	if s == nil {
		return nil
	}
	return s.err
}

func (s *stopChan) Wait() {
	if s == nil {
		return
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	for !s.stopped {
		s.cond.Wait()
	}
}

// The returned channel never receives any values, but is closed as soon as the
// underlying StopChannel is stopped. The Err() method can be used to retrieve the error instance afterwards.
func (s *stopChan) WaitChan() <-chan error {
	if s == nil {
		c := make(chan error)
		close(c)
		return c
	}
	// Double checked locking
	// To avoid memory leak, lazily create one channel and one goroutine.
	if s.waitChan == nil {
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
		if s.waitChan == nil {
			s.waitChan = make(chan error)
			c := s.waitChan
			go func() {
				s.Wait()
				close(c)
			}()
		}
	}
	return s.waitChan
}

// Return true means wait timed out, return false means the StopChan was stopped before the timeout expired.
func (s *stopChan) WaitTimeout(t time.Duration) bool {
	if s == nil {
		return false
	}
	select {
	case <-time.After(t):
		return true
	case <-s.WaitChan():
		return false
	}
}

func (s *stopChan) IfStopped(execute func()) {
	if s == nil {
		execute()
		return
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	if !s.stopped {
		return
	}
	execute()
}

func (s *stopChan) IfNotStopped(execute func()) {
	if s == nil {
		return
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	if s.stopped {
		return
	}
	execute()
}

func (s *stopChan) IfElseStopped(stopped func(), notStopped func()) {
	if s == nil {
		stopped()
		return
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	if s.stopped {
		stopped()
	} else {
		notStopped()
	}
}

// ========= Additional constructors =========

func WaitErrFunc(wg *sync.WaitGroup, wait func() error) StopChan {
	if wg != nil {
		wg.Add(1)
	}
	finished := NewStopChan()
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		var err error
		if wait != nil {
			err = wait()
		}
		finished.StopErr(err)
	}()
	return finished
}

func WaitFunc(wg *sync.WaitGroup, wait func()) StopChan {
	return WaitErrFunc(wg, func() error {
		wait()
		return nil
	})
}

// Like WaitErrFunc, but only close the resulting StopChan, if the setup function
// returns a non-nil error
func WaitForSetup(wg *sync.WaitGroup, setup func() error) StopChan {
	if wg != nil {
		wg.Add(1)
	}
	failed := NewStopChan()
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		if setup != nil {
			if err := setup(); err != nil {
				failed.StopErr(err)
			}
		}
	}()
	return failed
}

// ========= Helper functions =========

func WaitForAny(channels []StopChan) int {
	if len(channels) == 0 {
		return -1
	}
	// Use reflect package to wait for any of the given channels
	var cases []reflect.SelectCase
	for _, ch := range channels {
		if ch.stopChan != nil {
			waitChan := ch.WaitChan()
			refCase := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(waitChan)}
			cases = append(cases, refCase)
		}
	}
	if len(cases) == 0 {
		return -1
	}
	choice, _, _ := reflect.Select(cases)
	return choice
}
