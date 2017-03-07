package golib

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
)

// Task is an interface for coordinates setup and teardown of applications or objects.
// The Task interface defines a simple lifecycle. First, it is started through Start().
// It runs until it finishes or produces an error. After any of these, the StopChan returned
// from the Start() method must be stopped. When the Stop() method is called, the Task
// must shutdown on-demand. In that case, the StopChan must also be stopped.
//
// When the StopChan is stopped, the Task must be completely inactive, including exiting any
// goroutines that have been created for its operation.
type Task interface {

	// The Start() method will be called exactly once and should fully initialize and start
	// the underlying task. If this involves creating goroutines, they should be registered
	// in the given *sync.WaitGroup. The resulting StopChan must be closed when the task finishes,
	// optionally logging an error explaining the reason for stopping prematurely.
	// When stopping, all goroutines must exit as well, reducing the counter in the WaitGroup.
	//
	// Note: Returning the uninitialized nil-value (StopChan{}) indicates that the task
	// will never shut down on its own, i.e. the task is not capable of producing an error.
	//
	// If an error happens immediately inside the Start() method, NewStoppedChan(err) should be returned.
	Start(wg *sync.WaitGroup) StopChan

	// Stop should be idempotent and cause the underlying task to stop on-demand.
	// It might be called multiple times, and it might also be called if the StopChan
	// is stopped prematurely due to an error.
	// The StopChan returned from Start() must be stopped after the Task finishes,
	// regardless whether Stop() has been called or not.
	Stop()

	// String returns a concise and human-readable description of the task.
	String() string
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
	// state of the task, or to call WaitTimeout() to controll the frequency of the loop.
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
// This should not be user if the standard input is used for different purposes.
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
