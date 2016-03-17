package golib

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"reflect"
	"sync"
)

// ========= Task interface
type StopChan <-chan error

// Semantics: Start() may only be called once, but Stop() should be idempotent.
// One error must be sent on StopChan upon stopping. The error can be nil.
type Task interface {
	Start(wg *sync.WaitGroup) StopChan
	Stop()
}

type NoopTask struct {
	Chan        StopChan
	Description string
}

func (task *NoopTask) Start(*sync.WaitGroup) StopChan {
	return task.Chan
}
func (task *NoopTask) Stop() {
}
func (task *NoopTask) String() string {
	return fmt.Sprintf("Task(%v)", task.Description)
}

type CleanupTask struct {
	Cleanup func()
	once    sync.Once
}

func (task *CleanupTask) Start(*sync.WaitGroup) StopChan {
	return make(chan error, 1) // Never triggered
}
func (task *CleanupTask) Stop() {
	task.once.Do(func() {
		if cleanup := task.Cleanup; cleanup != nil {
			cleanup()
		}
	})
}

type loopTask struct {
	*OneshotCondition
	loop func(stop StopChan)
}

func (task *loopTask) Start(wg *sync.WaitGroup) StopChan {
	cond := task.OneshotCondition
	if loop := task.loop; loop != nil {
		stop := WaitCondition(wg, cond)
		if wg != nil {
			wg.Add(1)
		}
		go func() {
			if wg != nil {
				defer wg.Done()
			}
			for !cond.Enabled() {
				loop(stop)
			}
		}()
	}
	return cond.Start(wg)
}

func LoopTask(loop func(stop StopChan)) Task {
	return &loopTask{NewOneshotCondition(), loop}
}

// ========= Helpers to implement Task interface

func TaskFinished() StopChan {
	return TaskFinishedError(nil)
}

func TaskFinishedError(err error) StopChan {
	res := make(chan error, 1)
	res <- err
	return res
}

func WaitErrFunc(wg *sync.WaitGroup, wait func() error) StopChan {
	if wg != nil {
		wg.Add(1)
	}
	finished := make(chan error, 1)
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		var err error
		if wait != nil {
			err = wait()
		}
		finished <- err
		close(finished)
	}()
	return finished
}

func WaitFunc(wg *sync.WaitGroup, wait func()) StopChan {
	return WaitErrFunc(wg, func() error {
		wait()
		return nil
	})
}

func WaitCondition(wg *sync.WaitGroup, cond *OneshotCondition) StopChan {
	if cond == nil {
		return nil
	}
	return WaitFunc(wg, func() {
		cond.Wait()
	})
}

func WaitForAny(channels []StopChan) (int, error) {
	if len(channels) < 1 {
		return -1, nil
	}
	// Use reflect package to wait for any of the given channels
	var cases []reflect.SelectCase
	for _, ch := range channels {
		if ch != nil {
			refCase := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
			cases = append(cases, refCase)
		}
	}
	choice, result, _ := reflect.Select(cases)
	channels[choice] = nil // Already received
	return choice, result.Interface().(error)
}

func WaitForAnyTask(wg *sync.WaitGroup, tasks []Task) (Task, error, []StopChan) {
	channels := make([]StopChan, 0, len(tasks))
	for _, task := range tasks {
		if channel := task.Start(wg); channel != nil {
			channels = append(channels, channel)
		}
	}
	choice, err := WaitForAny(channels)
	return tasks[choice], err, channels
}

func CollectErrors(inputs []StopChan) []error {
	result := make([]error, 0, len(inputs))
	for _, input := range inputs {
		if err := <-input; err != nil {
			result = append(result, err)
		}
	}
	return result
}

func WaitForSetup(wg *sync.WaitGroup, setup func() error) StopChan {
	if wg != nil {
		wg.Add(1)
	}
	failed := make(chan error, 1)
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		if setup != nil {
			if err := setup(); err != nil {
				failed <- err
				close(failed)
			}
		}
	}()
	return failed
}

// ========= Task Group

type TaskGroup struct {
	names  []string          // Track order of added new groups
	groups map[string][]Task // Groups will be stopped sequentially, but Tasks in one group in parallel
	all    []Task
}

func NewTaskGroup(tasks ...Task) *TaskGroup {
	group := &TaskGroup{
		groups: make(map[string][]Task),
	}
	for _, o := range tasks {
		group.Add(o)
	}
	return group
}

func (group *TaskGroup) Add(tasks ...Task) {
	group.AddNamed("default", tasks...)
}

func (group *TaskGroup) AddNamed(name string, tasks ...Task) {
	if list, ok := group.groups[name]; ok {
		group.groups[name] = append(list, tasks...)
	} else {
		group.groups[name] = tasks
		group.names = append(group.names, name)
	}
	group.all = append(group.all, tasks...)
}

func (group *TaskGroup) WaitForAny(wg *sync.WaitGroup) (Task, error, []StopChan) {
	return WaitForAnyTask(wg, group.all)
}

func (group *TaskGroup) ReverseStop() {
	for i := len(group.names) - 1; i >= 0; i-- {
		// Stop groups in reverse order
		var wg sync.WaitGroup
		tasks := group.groups[group.names[i]]
		for _, task := range tasks {
			// Stop tasks in one group in parallel
			wg.Add(1)
			go func(task Task) {
				defer wg.Done()
				task.Stop()
			}(task)
		}
		wg.Wait()
	}
}

func (group *TaskGroup) WaitAndStop() (Task, []error) {
	var wg sync.WaitGroup
	choice, err, others := group.WaitForAny(&wg)
	group.ReverseStop()
	wg.Wait()
	errors := CollectErrors(others)
	errors = append(errors, err)
	return choice, errors
}

func (group *TaskGroup) PrintWaitAndStop() {
	reason, errors := group.WaitAndStop()
	log.Printf("Stopped because of %T: %v\n", reason, reason)
	for _, err := range errors {
		if err != nil {
			log.Println("Error:", err)
		}
	}
}

// ========= Sources of interrupts by the user

func ExternalInterrupt() StopChan {
	// This must be done after starting any openRTSP subprocess that depensd
	// the ignore-handler for SIGNIT provided by ./noint
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	stop := make(chan error)
	go func() {
		defer signal.Stop(interrupt)
		<-interrupt
		stop <- nil
	}()
	return stop
}

func UserInput() StopChan {
	userinput := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, err := reader.ReadString('\n')
		if err != nil {
			err = fmt.Errorf("Error reading user input: %v", err)
		}
		userinput <- err
	}()
	return userinput
}

func StdinClosed() StopChan {
	closed := make(chan error, 1)
	go func() {
		_, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			err = fmt.Errorf("Error reading stdin: %v", err)
		}
		closed <- err
	}()
	return closed
}
