package golib

import (
	"fmt"
	"os"
)

var (
	ErrorExitHook func()
	_exiting      bool
)

func Checkerr(err error) {
	if err != nil {
		if _exiting {
			Log.Warnln("Recursive golib.Checkerr:", err)
			return
		}
		logExit(err)
	}
}

func Fatalln(args ...interface{}) {
	if _exiting {
		args = append([]interface{}{"Recursive golib.Fatalln:"}, args...)
		Log.Warnln(args...)
		return
	}
	logExit(args...)
}

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

func Printerr(err error) {
	if err != nil {
		Log.Errorln(err)
	}
}
