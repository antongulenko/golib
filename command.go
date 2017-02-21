package golib

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type Command struct {
	Program string
	Args    []string

	// Can be set for more descriptive log messages
	ShortName string

	// Optional log files

	Logdir  string
	Logfile string

	// Fields below will be initialized in Start()

	Proc     *os.Process
	State    *os.ProcessState
	StateErr error

	processFinished StopChan
}

func (command *Command) Start(wg *sync.WaitGroup) StopChan {
	process := exec.Command(command.Program, command.Args...)
	if command.Logdir != "" && command.Logfile != "" {
		logF, err := openLogfile(command.Logdir, command.Logfile)
		if err != nil {
			return NewStoppedChan(err)
		}
		command.Logfile = logF.Name()
		process.Stdout = logF
		process.Stderr = logF
	} else {
		command.Logfile = ""
		process.Stdout = nil
		process.Stderr = nil
	}

	err := process.Start()
	if err != nil {
		return NewStoppedChan(err)
	}

	if command.ShortName == "" {
		command.ShortName = command.Program
	}
	command.processFinished = NewStopChan()
	command.Proc = process.Process

	wg.Add(1)
	go command.waitForProcess(wg)
	return command.processFinished
}

func openLogfile(dirname, filename string) (*os.File, error) {
	err := os.MkdirAll(dirname, os.FileMode(0775))
	if err != nil {
		return nil, err
	}
	logfile, err := ioutil.TempFile(dirname, filename)
	if err != nil {
		return nil, err
	}
	err = logfile.Truncate(0)
	if err != nil {
		return nil, err
	}
	return logfile, nil
}

func (command *Command) waitForProcess(wg *sync.WaitGroup) {
	defer wg.Done()
	state, err := command.Proc.Wait()
	if state == nil && err == nil {
		err = fmt.Errorf("No ProcState returned")
	}
	command.State, command.StateErr = state, err
	command.processFinished.StopErr(err)
}

func (command *Command) String() string {
	return command.StateString() + " (" + command.Logfile + ")"
}

func (command *Command) Stop() {
	if err := command.checkStarted(); err != nil {
		return
	}
	command.Proc.Signal(syscall.SIGHUP)
}

func (command *Command) IsFinished() bool {
	if err := command.checkStarted(); err != nil {
		return false
	}
	return command.processFinished.Stopped()
}

func (command *Command) Success() bool {
	return command.StateErr != nil || (command.State != nil && command.State.Success())
}

func (command *Command) StateString() string {
	if err := command.checkStarted(); err != nil {
		return err.Error()
	}
	if !command.IsFinished() {
		return fmt.Sprintf("%v (%v) running", command.ShortName, command.Proc.Pid)
	}
	if command.State == nil {
		return fmt.Sprintf("%v wait error: %s", command.ShortName, command.StateErr)
	} else {
		if command.State.Success() {
			return fmt.Sprintf("%v (%v) successful exit", command.ShortName, command.Proc.Pid)
		} else {
			return fmt.Sprintf("%v (%v) exit: %s", command.ShortName, command.Proc.Pid, command.State.String())
		}
	}
}

func (command *Command) checkStarted() error {
	if command == nil || command.Proc == nil {
		return fmt.Errorf("Command is nil or not started")
	}
	return nil
}
