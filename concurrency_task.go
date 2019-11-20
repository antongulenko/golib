package golib

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Startable objects can be started and notify a controlling instance when they finish.
// The main interface for this is the Task interface. The Startable interface is extracted
// to make the signature of the Start() method reusable.
type Startable interface {
	// The Start() method will be called exactly once and should fully initialize and start
	// the underlying task. If this involves creating goroutines, they should be registered
	// in the given *sync.WaitGroup. The resulting StopChan must be closed by the task when it finishes,
	// optionally logging an error explaining the reason for stopping prematurely.
	// When stopping, all goroutines must exit as well, reducing the counter in the WaitGroup.
	//
	// Note: Returning the uninitialized nil-value (StopChan{}) indicates that the task
	// will never shut down on its own, i.e. the task is not capable of producing an error.
	//
	// If an error happens immediately inside the Start() method, NewStoppedChan(err) should be returned.
	Start(wg *sync.WaitGroup) StopChan
}

// Task is an interface for coordinated setup and tear-down of applications or objects.
// The Task interface defines a simple lifecycle. First, it is started through Start().
// It runs until it finishes or produces an error. After any of these, the StopChan returned
// from the Start() method must be stopped by the task.
//
// When the StopChan is stopped, the Task must be completely inactive, including exiting any
// goroutines that have been created for its operation.
type Task interface {
	Startable

	// Stop should be idempotent and cause the underlying task to stop on-demand.
	// It might be called multiple times, and it might also be called if the StopChan
	// is stopped prematurely due to an error.
	// The StopChan returned from Start() must be stopped after the Task finishes,
	// regardless whether Stop() has been called or not.
	Stop()

	// String returns a concise and human-readable description of the task.
	String() string
}

// SetupTask is an implementation of the Task interface that executes a set routine
// when the task is started. The task itself does not do anything.
type SetupTask struct {
	// Setup will be executed when this Task is started.
	Setup func()
	// Description should be set to something that describes the purpose of this task.
	Description string
}

// Start implements the Task interface by executing the Setup routine and returning the nil-value of StopChan,
// which indicates that this task does not actively do anything.
func (task *SetupTask) Start(*sync.WaitGroup) StopChan {
	if setup := task.Setup; setup != nil {
		setup()
	}
	return StopChan{}
}

// Stop implements the Task interface and does nothing.
func (task *SetupTask) Stop() {
}

// String implements the Task interface by using the user-defined Description field.
func (task *SetupTask) String() string {
	return fmt.Sprintf("Setup(%s)", task.Description)
}

// CleanupTask is an implementation of the Task interface that executes a cleanup routine
// when the task is stopped. The task itself does not do anything.
type CleanupTask struct {
	// Cleanup will be executed when this Task is stopped.
	Cleanup func()
	// Description should be set to something that describes the purpose of this task.
	Description string
	once        sync.Once
}

// Start implements the Task interface by returning the nil-value of StopChan,
// which indicates that this task does not actively do anything.
func (task *CleanupTask) Start(*sync.WaitGroup) StopChan {
	return StopChan{}
}

// Stop implements the Task interface by executing the configured Cleanup function
// exactly once.
func (task *CleanupTask) Stop() {
	task.once.Do(func() {
		if cleanup := task.Cleanup; cleanup != nil {
			cleanup()
		}
	})
}

// String implements the Task interface by using the user-defined Description field.
func (task *CleanupTask) String() string {
	return fmt.Sprintf("Cleanup(%s)", task.Description)
}

// NoopTask is a trivial implementation of the Task interface that uses a user-defined
// instance of StopChan. It can be used to wrap arbitrary instances of StopChan
// in the Task interface.
type NoopTask struct {
	// Chan will be returns from the Start() method and stopped by the Stop() method.
	Chan StopChan
	// Description should be set to something that describes the purpose of this task.
	Description string
}

// Start returns the provided StopChan instance.
func (task *NoopTask) Start(*sync.WaitGroup) StopChan {
	return task.Chan
}

// Stop stops the provided StopChan instance.
func (task *NoopTask) Stop() {
	task.Chan.Stop()
}

// String returns a description of the task based on the user-defined Description field.
func (task *NoopTask) String() string {
	return fmt.Sprintf("Task(%v)", task.Description)
}

// LoopTask is an implementation of the Task interface that spawns a worker goroutine
// that executes a loop until the task is stopped or an error is encountered.
type LoopTask struct {
	// StopChan is added as an anonymous field, which allows direct access to the
	// Stop() method and other methods that control the execution of the loop.
	StopChan

	// Description should be set to something that describes the purpose of this loop task.
	Description string

	// StopHook can optionally be set to a callback function, which will be executed
	// after the loop is finished (but before the underlying StopChan is stopped).
	StopHook func()

	// Loop defines the loop iteration. The stop parameter can be used to query the current
	// state of the task, or to call WaitTimeout() to control the frequency of the loop.
	// If the return value is non-nil, the task will be stopped. If the return value
	// is StopLoopTask, the task will be stopped without reporting an error.
	Loop func(stop StopChan) error
}

// StopLoopTask can be returned from the LoopTask.Loop function to make the loop task
// stop without reporting an error.
var StopLoopTask = errors.New("Stop the LoopTask")

// Start implements the Task interface by spawning a goroutine that executes a loop
// until the task is stopped or the loop iteration returns an error.
func (task *LoopTask) Start(wg *sync.WaitGroup) StopChan {
	task.StopChan = NewStopChan()
	stop := task.StopChan

	if loop := task.Loop; loop != nil {
		if wg != nil {
			wg.Add(1)
		}
		go func() {
			if wg != nil {
				defer wg.Done()
			}
			if hook := task.StopHook; hook != nil {
				defer hook()
			}
			for !stop.Stopped() {
				err := loop(stop)
				if err != nil {
					if err == StopLoopTask {
						err = nil
					}
					stop.StopErr(err)
				}
			}
		}()
	}
	return stop
}

// String returns a description of the task using the user-defined Description value.
func (task *LoopTask) String() string {
	return fmt.Sprintf("LoopTask(%s)", task.Description)
}

// TimeoutTask is a Task that automatically fails after a predefined time.
// If the task is stopped before the timeout expires, no errors is logged.
// If DumpGoroutines is set to true, all running goroutines will be printed when
// a timeout occurs.
type TimeoutTask struct {
	Timeout        time.Duration
	ErrorMessage   string
	DumpGoroutines bool

	stopper StopChan
}

// Start implements the Task interface
func (t *TimeoutTask) Start(wg *sync.WaitGroup) StopChan {
	t.stopper = NewStopChan()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if t.stopper.WaitTimeout(t.Timeout) {
			if t.DumpGoroutines {
				DumpGoroutineStacks()
			}
			msg := fmt.Sprintf("Timeout after %v", t.Timeout)
			if t.ErrorMessage != "" {
				msg += ": " + t.ErrorMessage
			}
			t.stopper.StopErr(errors.New(msg))
		}
	}()
	return t.stopper
}

// Stop implements the Task interface
func (t *TimeoutTask) Stop() {
	t.stopper.Stop()
}

// Stop implements the Task interface
func (t *TimeoutTask) String() string {
	msg := fmt.Sprintf("Timeout (%v)", t.Timeout)
	if t.ErrorMessage != "" {
		msg += ": " + t.ErrorMessage
	}
	return msg
}

// ExternalInterruptTask returns a Task that automatically stops when
// the SIGINT signal is received (e.g. by pressing Ctrl-C).
func ExternalInterruptTask() *NoopTask {
	return &NoopTask{
		Chan:        ExternalInterrupt(),
		Description: "SIGINT received",
	}
}

// UserInputTask returns a Task that automatically stops when a newline
// character is received on the standard input (See UserInput()).
func UserInputTask() *NoopTask {
	return &NoopTask{
		Chan:        UserInput(),
		Description: "Newline received on stdin",
	}
}

// StdinClosedTask returns a Task that automatically stops when the standard
// input stream is closed.
func StdinClosedTask() *NoopTask {
	return &NoopTask{
		Chan:        StdinClosed(),
		Description: "Stdin closed",
	}
}
