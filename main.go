package golib

import (
	"fmt"
	"os"
	"runtime/pprof"
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
