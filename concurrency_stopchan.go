package golib

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
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

// StopChan is a utility type for coordinating concurrent goroutines.
// Initially, a StopChan is 'running' and can be stopped exactly once.
// Goroutines can wait for the StopChan to be stopped and query the current status
// in various ways.
//
// When stopping a StopChan, an error instance can optionally be stored for
// later reference.
//
// StopChan values should always be passed and stored by-value instead of by-reference,
// since they contain a pointer to the actual internal data.
//
// The nil-value of StopChan (e.g. StopChan{}) mostly acts like an already-stopped StopChan with a nil error.
// The only exception is the WaitForAny() function, which will ignore uninitialized StopChans.
type StopChan struct {
	*stopChan
}

// NewStopChan allocates a new, un-stopped StopChan.
func NewStopChan() StopChan {
	return StopChan{
		stopChan: &stopChan{
			cond: sync.Cond{
				L: new(sync.Mutex),
			},
		},
	}
}

// NewStoppedChan returns a StopChan that is already stopped, and contains the
// given error value.
func NewStoppedChan(err error) StopChan {
	res := NewStopChan()
	res.StopErr(err)
	return res
}

// IsNil returns true, if the receiving StopChan is its nil-values. See the StopChan
// documentation for details on the behavior of the nil value.
func (s *stopChan) IsNil() bool {
	return s == nil
}

// StopErrFunc stops the receiving StopChan, iff it is not already stopped.
// In that case, the given function is executed and the resulting error value
// is stored within the StopChan.
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

// StopFunc stops the receiving StopChan and executes the given function, iff
// it was not already stopped.
func (s *stopChan) StopFunc(perform func()) {
	s.StopErrFunc(func() error {
		if perform != nil {
			perform()
		}
		return nil
	})
}

// StopErr stops the receiving StopChan, iff it was not already stopped.
// The given error value is stored in the StopChan.
func (s *stopChan) StopErr(err error) {
	s.StopErrFunc(func() error {
		return err
	})
}

// Stop stops the receiving StopChan without storing any error value.
func (s *stopChan) Stop() {
	s.StopErrFunc(nil)
}

// Stopped returns whether the StopChan is stopped or not. It blocks, if the
// StopChan is currently being stopped by another goroutine.
func (s *stopChan) Stopped() bool {
	if s == nil {
		return true
	}
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	return s.stopped
}

// Err returns the error value stored in the StopChan. It will always be nil,
// if the StopChan has not been stopped yet, but can also be nil for a stopped StopChan.
func (s *stopChan) Err() error {
	if s == nil {
		return nil
	}
	return s.err
}

// Wait blocks until the receiving StopChan is stopped.
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

// WaitChan returns a channel that is closed as soon as the receiving StopChan
// is stopped. The returned channel never receives any values.
// The Err() method can be used to retrieve the error instance stored in the
// StopChan afterwards.
//
// To avoid memory leaks, only one channel is lazily created per StopChan instance,
// accompanied by one goroutine that closes that channel after waiting for the StopChan
// to be stopped. The same channel will be returned by all calls to WaitChan().
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

// WaitTimeout waits for the StopChan to be stopped, but returns if the given
// time duration has passed without that happening.
// The return value indicates which one of the two happened:
//  1. Return true means the wait timed out and the StopChan is still active.
//  2. Return false means the StopChan was stopped before the timeout expired.
func (s *stopChan) WaitTimeout(t time.Duration) bool {
	return s.WaitTimeoutPrecise(t, 1, nil)
}

// WaitTimeoutLoop behaves like WaitTimeout, but tries to achieve a more precise timeout timing
// by waking up frequently and checking the passed sleep time.
// The wakeupFactor parameter must be in ]0..1] or it is adjusted to 1. It is multiplied with the totalDuration
// to determine the sleep duration of intermediate sleeps for increasing the sleep duration accuracy in high-load situations.
// For example, a wakeupFactor of 0.1 will lead to 10 intermediate wake-ups that check if the desired sleep time has passed already.
// If the lastTime parameter is not nil and not zero, the sleep time will be counted not from time.Now(), but from the stored time.
// If the lastTime parameter is not zero, the current time is stored into it before returning.
func (s *stopChan) WaitTimeoutPrecise(totalTimeout time.Duration, wakeupFactor float64, lastTimePointer *time.Time) bool {
	if s == nil {
		return false
	}
	now := time.Now()

	var lastTime time.Time
	if lastTimePointer != nil {
		defer func() {
			// The now time might be changed in the loop below
			*lastTimePointer = now
		}()
		lastTime = *lastTimePointer
	}

	var end time.Time
	if lastTime.IsZero() || now.Before(lastTime) {
		end = now.Add(totalTimeout)
	} else {
		end = lastTime.Add(totalTimeout)
		totalTimeout = end.Sub(now)
		if totalTimeout <= 0 {
			return !s.Stopped()
		}
	}

	waitChan := s.WaitChan()
	if wakeupFactor <= 0 || wakeupFactor > 1 {
		wakeupFactor = 1
	}
	subTimeout := time.Duration(float64(totalTimeout) * wakeupFactor)

	for {
		select {
		case <-time.After(subTimeout):
			now = time.Now()
			leftTime := end.Sub(now)
			if leftTime <= 0 {
				return true
			} else if leftTime < subTimeout {
				subTimeout = leftTime
			}
		case <-waitChan:
			return false
		}
	}
}

// Execute executes the given function while grabbing the internal lock of the StopChan.
// This means that no other goroutine can stop the StopChan while the function is running,
// and that it is mutually exclusive with any of the IfStopped etc. methods.
// This is sometimes useful, if the StopChan is used for its locking capabilities.
func (s *stopChan) Execute(execute func()) {
	if s != nil {
		s.cond.L.Lock()
		defer s.cond.L.Unlock()
	}
	execute()
}

// IfStopped executes the given function, iff the receiving StopChan is not yet
// stopped. This call guarantees that the StopChan is not stopped while the
// function is being executed.
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

// IfNotStopped executes the given function, iff the receiving StopChan is already
// stopped. If another goroutine is currently stopping this StopChan (see StopErrFunc),
// IfNotStopped waits until the StopChan is finally stopped before executing the callback.
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

// IfElseStopped executes one of the two given functions, depending on the stopped state
// of the StopChan. This call guarantees that the StopChan is not stopped while any of the
// functions is being executed.
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

// WaitErrFunc executes the given function and returns a StopChan, that
// will automatically be stopped after the function finishes.
// The error instance return by the function will be stored in the StopChan.
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

// WaitErrFunc executes the given function and returns a StopChan, that
// will automatically be stopped after the function finishes.
func WaitFunc(wg *sync.WaitGroup, wait func()) StopChan {
	return WaitErrFunc(wg, func() error {
		wait()
		return nil
	})
}

// WaitForSetup executes the given function and returns a StopChan,
// that will be stopped after the function finishes, but ONLY if the
// function returns a non-nil error value. In that case the returned error
// is stored in the stopped StopChan.
//
// This behaviour is similar to WaitErrFunc, but it leaves the StopChan
// active if the setup function finished successfully.
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

// WaitForAny returns if any of the give StopChan values are stopped. The implementation/
// uses the reflect package to create a select-statement of variable size.
//
// Exception: Uninitialized StopChans (created through the nil-value StopChan{}) are ignored,
// although they behave like stopped StopChans otherwise.
//
// The return value is the index of the StopChan that caused this function to return.
// If the given channel slice is empty, or if it contains only uninitialized StopChan instances,
// the return value will be -1.
func WaitForAny(channels []StopChan) int {
	if len(channels) == 0 {
		return -1
	}
	// Use reflect package to wait for any of the given channels
	var cases []reflect.SelectCase
	validCases := 0
	for _, ch := range channels {
		var waitChan <-chan error
		if ch.stopChan == nil {
			// Channel that never returns. This dummy channel is used to make the
			// return value consistent (index of the channel closed in the select statement)
			waitChan = make(chan error, 1)
		} else {
			validCases++
			waitChan = ch.WaitChan()
		}

		refCase := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(waitChan)}
		cases = append(cases, refCase)
	}
	if validCases == 0 {
		return -1
	}
	choice, _, _ := reflect.Select(cases)
	return choice
}

// ExternalInterrupt creates a StopChan that is automatically stopped as soon
// as an interrupt signal (like pressing Ctrl-C) is received.
// This can be used in conjunction with the NoopTask to create a task
// that automatically stops when the process receives an interrupt signal.
func ExternalInterrupt() StopChan {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	stop := NewStopChan()
	go func() {
		defer signal.Stop(interrupt)
		select {
		case <-interrupt:
			stop.Stop()
		case <-stop.WaitChan():
		}
	}()
	return stop
}

// UserInput creates a StopChan that is automatically stopped when the
// a newline character is received on os.Stdin.
// This can be used in conjunction with the NoopTask to create a task
// that automatically stops when the user presses the enter key.
// This should not be used if the standard input is used for different purposes.
func UserInput() StopChan {
	userInput := NewStopChan()
	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, err := reader.ReadString('\n')
		if err != nil {
			err = fmt.Errorf("Error reading user input: %v", err)
		}
		userInput.StopErr(err)
	}()
	return userInput
}

// StdinClosed creates a StopChan that is automatically stopped when the
// standard input stream is closed.
// This can be used in conjunction with the NoopTask to create a task
// that automatically stops when the user presses Ctrl-D or stdin is closed for any other reason.
// This should not be user if the standard input is used for different purposes.
func StdinClosed() StopChan {
	closed := NewStopChan()
	go func() {
		_, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			err = fmt.Errorf("Error reading stdin: %v", err)
		}
		closed.StopErr(err)
	}()
	return closed
}
