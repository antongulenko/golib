package golib

import (
	"flag"
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

var (
	TaskStopTimeout   = time.Duration(0)
	PrintTaskStopWait = false
)

func RegisterTaskFlags() {
	flag.BoolVar(&PrintTaskStopWait, "debug-task-stop", PrintTaskStopWait, "Print tasks waited for when stopping (for debugging)")
	flag.DurationVar(&TaskStopTimeout, "debug-task-timeout", TaskStopTimeout, "Timeout duration when stopping and waiting for tasks to finish")
}

type TaskGroup []Task

func (group *TaskGroup) Add(tasks ...Task) {
	*group = append(*group, tasks...)
}

func (group TaskGroup) StartTasks(wg *sync.WaitGroup) []StopChan {
	channels := make([]StopChan, len(group))
	for i, task := range group {
		channels[i] = task.Start(wg)
	}
	return channels
}

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

func (group TaskGroup) PrintWaitAndStop() int {
	reason, numErrors := group.WaitAndStop(TaskStopTimeout)
	Log.Debugln("Stopped because of", reason)
	return numErrors
}

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

func (group TaskGroup) CollectMultiError(channels []StopChan) MultiError {
	var err MultiError
	group.CollectErrors(channels, func(newErr error) {
		err.Add(newErr)
	})
	return err
}
