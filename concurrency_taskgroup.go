package golib

import (
	"flag"
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

var (
	// TaskStopTimeout is used by PrintWaitAndStop() to timeout the shutdown sequence,
	// dump all running goroutines and exiting when shutting down takes too long.
	TaskStopTimeout = time.Duration(0)

	// PrintTaskStopWait causes various parts of the TaskGroup functionality to print
	// debug messages when stopping tasks and waiting for them to finish.
	PrintTaskStopWait = false
)

// RegisterTaskFlags registers flags for controlling the global variables
// TaskStopTimeout and PrintTaskStopWait, which can be used to debug shutdown sequences
// when using TaskGroups.
func RegisterTaskFlags() {
	flag.BoolVar(&PrintTaskStopWait, "debug-task-stop", PrintTaskStopWait, "Print tasks waited for when stopping (for debugging)")
	flag.DurationVar(&TaskStopTimeout, "debug-task-timeout", TaskStopTimeout, "Timeout duration when stopping and waiting for tasks to finish")
}

// TaskGroup is a collection of tasks that can be started and stopped together.
// The purpose of this type is to coordinate the startup and shutdown sequences
// of multiple parts of one application or object.
type TaskGroup []Task

// Add adds the given tasks to the task group.
func (group *TaskGroup) Add(tasks ...Task) {
	*group = append(*group, tasks...)
}

// StartTasks starts all tasks in the task group and returns the created
// StopChan instances in the same order as the tasks.
func (group TaskGroup) StartTasks(wg *sync.WaitGroup) []StopChan {
	channels := make([]StopChan, len(group))
	for i, task := range group {
		channels[i] = task.Start(wg)
	}
	return channels
}

// Stop stops all tasks in the task group in parallel.
// Stop blocks until all Stop() invocations of all tasks have returned.
//
// If the global PrintTaskStopWait variable is set, a log message
// is printed before stopping every task.
func (group TaskGroup) Stop() {
	var wg sync.WaitGroup
	for _, task := range group {
		wg.Add(1)
		go func(task Task) {
			defer wg.Done()
			if PrintTaskStopWait {
				Log.Println("Stopping", task)
			}
			task.Stop()
		}(task)
	}
	wg.Wait()
}

// WaitAndStop executes the entire lifecycle sequence for all tasks in the task group:
// - Start all tasks using StartTasks() with a new instance of sync.WaitGroup
// - Wait for the first task to finish
// - Stop all tasks using Stop()
// - Wait until all goroutines end using sync.WaitGroup.Wait()
// - Wait until all tasks finish using CollectErrors()
//
// All errors produced by any task are logged.
// Afterwards, the task that caused the shutdown is returned, as well as the number
// of errors encountered.
//
// If the timeout parameters is >0, all goroutines will be dumped to the standard output
// and the program will terminate. This can be used to debug the task shutdown sequence,
// in case one task does not shut down properly.
func (group TaskGroup) WaitAndStop(timeout time.Duration) (Task, int) {
	var wg sync.WaitGroup
	channels := group.StartTasks(&wg)
	reason := WaitForAny(channels)
	if timeout > 0 {
		time.AfterFunc(timeout, func() {
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)
			panic("Waiting for stopping goroutines timed out")
		})
	}
	group.Stop()
	wg.Wait()

	numErrors := group.CollectErrors(channels, func(err error) {
		Log.Errorln(err)
	})
	return group[reason], numErrors
}

// PrintWaitAndStop calls WaitAndStop() using the global variable TaskStopTimeout
// as the timeout parameter. Afterwards, the task that caused the shutdown is printed
// as a debug log-message and the number of errors encountered is returned.
// This is a convenience function that can be used in main() functions.
func (group TaskGroup) PrintWaitAndStop() int {
	reason, numErrors := group.WaitAndStop(TaskStopTimeout)
	Log.Debugln("Stopped because of", reason)
	return numErrors
}

// CollectErrors waits for the given StopChan instances to stop and calls the given
// callback function for every collected non-nil error instance.
//
// The channels slice must be the one created by StartTasks().
//
// If the global PrintTaskStopWait variable is true, and additional log message
// is printed when starting waiting for a task. This can be used to identify the task
// that prevents the shutdown from progressing.
func (group TaskGroup) CollectErrors(channels []StopChan, do func(err error)) (numErrors int) {
	for i, input := range channels {
		if input.stopChan != nil {
			if PrintTaskStopWait {
				task := group[i]
				Log.Println("Waiting for", task)
			}
			input.Wait()
			if err := input.Err(); err != nil {
				numErrors++
				do(err)
			}
		}
	}
	return
}

// CollectMultiError uses CollectErrors to collect all errors returned from all StopChan instances
// into one MultiError object.
//
// The channels slice must be the one created by StartTasks().
func (group TaskGroup) CollectMultiError(channels []StopChan) MultiError {
	var err MultiError
	group.CollectErrors(channels, func(newErr error) {
		err.Add(newErr)
	})
	return err
}
