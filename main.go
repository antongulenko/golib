package golib

import (
	"os"
	"runtime/pprof"

	"github.com/kballard/go-shellquote"
)

// Checkerr stops the process with a non-zero exit status, if the given error
// is non-nil. Before exiting, it executes the ErrorExitHook function, if it is defined.
func Checkerr(err error) {
	if err != nil {
		Log.Fatalln(err)
	}
}

// Printerr prints an error message, if the given error is non-nil.
func Printerr(err error) {
	if err != nil {
		Log.Errorln(err)
	}
}

func DumpGoroutineStacks() {
	_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)
}

// ParseHashbangArgs checks, if the current process was started in one of the following forms:
//   /path/to/EXECUTABLE executable-script-file <additional args>...
//   EXECUTABLE "-flag1 -flag2 arg1 arg2" executable-script-file <additional args>...
// These forms are used by the OS when running an executable script that has a first line like one of the following:
//   #!/usr/bin/env EXECUTABLE
//   #!/path/to/EXECUTABLE -flag1 -flag2 arg1 arg2
// The <additional args> are passed to the process from the command line when executing the hashbang script.
//
// The hashbang execution is determined by checking if the first or second parameter is an executable file.
// If the executable file is on the second parameter, the first parameter is split based on syntax rules of /bin/sh,
// and the modified arguments are written back into the given slice (which usually should be &os.Args).
// This allows specifying multiple parameters in the hashbang header of a script, which are usually passed into the
// executable as one single parameter string.
//
// The return value is the index of the script file in the argument slice. If it is 0, no hashbang execution was detected.
func ParseHashbangArgs(argsPtr *[]string) int {
	args := *argsPtr
	if len(args) <= 1 {
		return 0
	}

	var scriptIndex int
	if IsExecutable(args[1]) {
		scriptIndex = 1
	} else if len(args) >= 3 && IsExecutable(args[2]) {
		scriptIndex = 2
	} else {
		return 0
	}
	if scriptIndex == 2 {
		// The second hashbang format was used: split the additional parameter string into separate arguments
		scriptFile := args[scriptIndex]
		splitArgs, err := shellquote.Split(args[1])
		if err == nil {
			// Silently ignore parsing error and don't adjust os.Args
			scriptIndex = 1 + len(splitArgs)
			splitArgs = append(splitArgs, scriptFile)
			splitArgs = append(splitArgs, args[3:]...)
			*argsPtr = append(args[0:1], splitArgs...)
		}
	}
	return scriptIndex
}

// IsExecutable returns true, if the given file is a regular file with the executable flag enabled.
func IsExecutable(filename string) bool {
	info, err := os.Stat(filename)
	return err == nil && info.Mode().IsRegular() && (info.Mode()&0111) != 0
}
