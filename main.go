package golib

import (
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/kballard/go-shellquote"
)

var (
	// ErrorExitHook will be executed when any of the Checkerr, Fatalln or Fatalf,
	// functions cause the current process to stop.
	ErrorExitHook func()
	_exiting      bool
)

// Checkerr stops the process with a non-zero exit status, if the given error
// is non-nil. Before exiting, it executes the ErrorExitHook function, if it is defined.
func Checkerr(err error) {
	if err != nil {
		if _exiting {
			Log.Warnln("Recursive golib.Checkerr:", err)
			return
		}
		logExit(err)
	}
}

// Fatalln prints an error log message and stops the process with a non-zero exit status.
// Before exiting, it executes the ErrorExitHook function, if it is defined.
func Fatalln(args ...interface{}) {
	if _exiting {
		args = append([]interface{}{"Recursive golib.Fatalln:"}, args...)
		Log.Warnln(args...)
		return
	}
	logExit(args...)
}

// Fatalf prints a formatted error log message and stops the process with a non-zero exit status.
// Before exiting, it executes the ErrorExitHook function, if it is defined.
func Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if _exiting {
		Log.Warnln("Recursive golib.Fatalf: " + msg)
		return
	}
	logExit()
}

func logExit(args ...interface{}) {
	_exiting = true
	args = append([]interface{}{"Fatal:"}, args...)
	Log.Errorln(args...)
	if ErrorExitHook != nil {
		ErrorExitHook()
	}
	os.Exit(1)
}

// Printerr prints an error message, if the given error is non-nil.
func Printerr(err error) {
	if err != nil {
		Log.Errorln(err)
	}
}

func DumpGoroutineStacks() {
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)
}

// IsHashbangExecution checks, if the current process was started in one of the following forms:
//   /path/to/EXECUTABLE executable-script-file
//   EXECUTABLE "-flag1 -flag2 arg1 arg2" executable-script-file
// These forms are used by the OS when running an executable script that has a first line like one of the following:
//   #!/usr/bin/env EXECUTABLE
//   #!/path/to/EXECUTABLE -flag1 -flag2 arg1 arg2
func IsHashbangExecution() bool {
	if len(os.Args) == 2 || len(os.Args) == 3 {
		scriptFile := os.Args[len(os.Args)-1]
		info, err := os.Stat(scriptFile)
		return err == nil && info.Mode().IsRegular() && (info.Mode()&0111) != 0
	}
	return false
}

// ParseHashbangArgs should be invoked when IsHashbangExecution() returns true. In case of two parameters,
// the first parameter is split based on syntax rules of /bin/sh, and the modified arguments are written back into
// os.Args. This allows specifying multiple parameters in the hashbang header of a script, which are usually passed into
// the executable as one single parameter string.
func ParseHashbangArgs() error {
	if len(os.Args) == 3 {
		scriptFile := os.Args[len(os.Args)-1]
		splitArgs, err := shellquote.Split(os.Args[1])
		if err != nil {
			return err
		}
		splitArgs = append(splitArgs, scriptFile)
		os.Args = append(os.Args[0:1], splitArgs...)
	}
	return nil
}
