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

// ========= Task interface =========

// Semantics: Start() may only be called once, but Stop() should be idempotent.
// StopChan must be stopped after the Task finishes, regardless whether Stop() has been called or not.
type Task interface {
	Start(wg *sync.WaitGroup) StopChan
	Stop()
	String() string // Tasks are frequently printed
}

// ========= CleanupTask =========

type CleanupTask struct {
	Cleanup     func()
	Description string
	once        sync.Once
}

func (task *CleanupTask) Start(*sync.WaitGroup) StopChan {
	return StopChan{}
}

func (task *CleanupTask) Stop() {
	task.once.Do(func() {
		if cleanup := task.Cleanup; cleanup != nil {
			cleanup()
		}
	})
}

func (task *CleanupTask) String() string {
	return fmt.Sprintf("Cleanup(%s)", task.Description)
}

// ========= NoopTask =========

type NoopTask struct {
	Chan        StopChan
	Description string
}

func (task *NoopTask) Start(*sync.WaitGroup) StopChan {
	return task.Chan
}

func (task *NoopTask) Stop() {
	task.Chan.Stop()
}

func (task *NoopTask) String() string {
	return fmt.Sprintf("Task(%v)", task.Description)
}

// ========= LoopTask =========

type LoopTask struct {
	StopChan
	Description string
	StopHook    func()
	Loop        func(stop StopChan) error
}

// Can be returned from the Loop func to make the loop stop without reporting an error
var StopLoopTask = errors.New("Stop the LoopTask")

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

func (task *LoopTask) String() string {
	return fmt.Sprintf("LoopTask(%s)", task.Description)
}

// ========= Sources of interrupts by the user
// Can be used with NoopTask to shut down a TaskGroup.

func ExternalInterrupt() StopChan {
	// This must be done after starting any subprocess that depends
	// on the ignore-handler for SIGNIT provided by ./noint
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

func UserInput() StopChan {
	userinput := NewStopChan()
	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, err := reader.ReadString('\n')
		if err != nil {
			err = fmt.Errorf("Error reading user input: %v", err)
		}
		userinput.StopErr(err)
	}()
	return userinput
}

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
