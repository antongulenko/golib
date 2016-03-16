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

// Semantics: Start() may only be called once, but Stop() should be idempotent.
type Task interface {
	Start(wg *sync.WaitGroup) <-chan interface{}
	Stop()
}

type NoopTask struct {
	Chan        <-chan interface{}
	Description string
}

func (task *NoopTask) Start(*sync.WaitGroup) <-chan interface{} {
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

func (task *CleanupTask) Start(*sync.WaitGroup) <-chan interface{} {
	return make(chan interface{}, 1) // Never triggered
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
	loop func(stop <-chan interface{})
}

func (task *loopTask) Start(wg *sync.WaitGroup) <-chan interface{} {
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

func LoopTask(loop func(stop <-chan interface{})) Task {
	return &loopTask{NewOneshotCondition(), loop}
}

// ========= Helpers to implement Task interface

func WaitFunc(wg *sync.WaitGroup, wait func()) <-chan interface{} {
	if wg != nil {
		wg.Add(1)
	}
	finished := make(chan interface{}, 1)
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		if wait != nil {
			wait()
		}
		finished <- nil
		close(finished)
	}()
	return finished
}

func WaitCondition(wg *sync.WaitGroup, cond *OneshotCondition) <-chan interface{} {
	if cond == nil {
		return nil
	}
	return WaitFunc(wg, func() {
		cond.Wait()
	})
}

func WaitForAny(channels []<-chan interface{}) int {
	if len(channels) < 1 {
		return -1
	}
	// Use reflect package to wait for any of the given channels
	var cases []reflect.SelectCase
	for _, ch := range channels {
		if ch != nil {
			refCase := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
			cases = append(cases, refCase)
		}
	}
	choice, _, _ := reflect.Select(cases)
	return choice
}

func WaitForAnyTask(wg *sync.WaitGroup, tasks []Task) Task {
	channels := make([]<-chan interface{}, 0, len(tasks))
	for _, task := range tasks {
		if channel := task.Start(wg); channel != nil {
			channels = append(channels, channel)
		}
	}
	choice := WaitForAny(channels)
	return tasks[choice]
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

func (group *TaskGroup) WaitForAny(wg *sync.WaitGroup) Task {
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

func (group *TaskGroup) WaitAndStop() Task {
	var wg sync.WaitGroup
	choice := group.WaitForAny(&wg)
	group.ReverseStop()
	wg.Wait()
	return choice
}

// ========= Sources of interrupts by the user

func ExternalInterrupt() <-chan interface{} {
	// This must be done after starting any openRTSP subprocess that depensd
	// the ignore-handler for SIGNIT provided by ./noint
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	stop := make(chan interface{})
	go func() {
		defer signal.Stop(interrupt)
		<-interrupt
		stop <- nil
	}()
	return stop
}

func UserInput() <-chan interface{} {
	userinput := make(chan interface{}, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading user input:", err)
		}
		userinput <- nil
	}()
	return userinput
}

func StdinClosed() <-chan interface{} {
	closed := make(chan interface{}, 1)
	go func() {
		_, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Println("Error reading stdin:", err)
		}
		closed <- nil
	}()
	return closed
}
