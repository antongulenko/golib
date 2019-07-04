package golib

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// Command starts a subprocess and optionally redirects the stdout and stderr
// streams to a log file.
// Command implements implements the Task interface: the subprocess
// can be stopped on demand, and the StopChan returns from the Start()
// method will be closed after the subprocess exists.
type Command struct {
	// Program is name of the executable that will be started as a subprocess
	Program string
	// Args are the arguments that will be passed to the spawned subprocess.
	Args []string

	// ShortName can optionally be set to a concise string describing the command
	// to make log messages more descriptive. Otherwise, the value of the Program field will be used.
	ShortName string

	// LogDir can be set together with Logfile to redirect the stderr and stdout
	// streams of the subprocess. A suffix is a appended to the given file to make
	// sure it does not exist.
	LogDir string
	// See LogDir
	LogFile string

	// PreserveStdout set to true will lead the subprocess to redirect its stdout and stderr streams to the
	// streams of the parent process (which is the default when launching processes). This flag is ignored when
	// LogDir and LogFile is set.
	PreserveStdout bool

	// Proc will be initialized when calling Start() and points to the running subprocess.
	Proc *os.Process

	// State and StateErr will be initialized when the subprocess exits and give information
	// about the exit state of the process.
	State *os.ProcessState
	// See State
	StateErr error

	processFinished StopChan
}

// Start implements the Task interface. It starts the process and returns a StopChan,
// that will be closed after the subprocess exits.
func (command *Command) Start(wg *sync.WaitGroup) StopChan {
	process := exec.Command(command.Program, command.Args...)
	if command.LogDir != "" && command.LogFile != "" {
		logF, err := openLogfile(command.LogDir, command.LogFile)
		if err != nil {
			return NewStoppedChan(err)
		}
		command.LogFile = logF.Name()
		process.Stdout = logF
		process.Stderr = logF
	} else {
		command.LogFile = ""
		command.LogDir = ""
		if command.PreserveStdout {
			process.Stdout = os.Stdout
			process.Stderr = os.Stderr
		} else {
			process.Stdout = nil
			process.Stderr = nil
		}
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
		err = errors.New("No ProcState returned")
	}
	command.State, command.StateErr = state, err
	command.processFinished.StopErr(err)
}

// String returns readable information about the process state and the
// logfile that contains stdout and stderr.
func (command *Command) String() string {
	state := command.StateString()
	if command.LogFile != "" {
		state += " (" + command.LogFile + ")"
	}
	return state
}

// Stop implements the Task interface and tries to stop the subprocess by
// sending it the SIGHUP signal.
// TODO try other measures to kill the subprocess, if it does not finish after a
// timeout.
func (command *Command) Stop() {
	if err := command.checkStarted(); err != nil {
		return
	}
	command.Proc.Signal(syscall.SIGHUP)
}

// IsFinished returns true if the subprocess has been started and then exited afterwards.
func (command *Command) IsFinished() bool {
	if err := command.checkStarted(); err != nil {
		return false
	}
	return command.processFinished.Stopped()
}

// Success returns true, if the subprocess has been started and finished successfully.
func (command *Command) Success() bool {
	return command.StateErr != nil || (command.State != nil && command.State.Success())
}

// StateString returns a descriptive string about the state of the subprocess.
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
		return errors.New("Command is nil or not started")
	}
	return nil
}
