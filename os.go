package golib

import (
	"flag"
	"syscall"
	"unsafe"
)

var (
	// ConfiguredOpenFilesLimit is used by ConfigureOpenFilesLimit() to try to configure allowed the number
	// of open files for the current process. It is set by the '-ofl' flag created by the RegisterOFLFlags()
	// function.
	ConfiguredOpenFilesLimit uint64
)

// RegisterOFLFlags registers a flag to configure the ConfiguredOpenFilesLimit global variable.
func RegisterOFLFlags() {
	flag.Uint64Var(&ConfiguredOpenFilesLimit, "ofl", ConfiguredOpenFilesLimit,
		"Set to >0 for configuring the open files limit (only possible as root)")
}

// ConfigureOpenFilesLimit calls SetOpenFilesLimit() with ConfiguredOpenFilesLimit as parameter,
// if ConfiguredOpenFilesLimit is set to a values > 0.
func ConfigureOpenFilesLimit() {
	if ConfiguredOpenFilesLimit > 0 {
		if err := SetOpenFilesLimit(ConfiguredOpenFilesLimit); err != nil {
			Log.Warnln("Failed to set open files limit to %v: %v", ConfiguredOpenFilesLimit, err)
		} else {
			Log.Println("Successfully set open files limit to %v", ConfiguredOpenFilesLimit)
		}
	}
}

// SetOpenFilesLimit tries to set the maximum number of open files for the current process to the
// given number.
func SetOpenFilesLimit(ulimit uint64) error {
	rLimit := syscall.Rlimit{
		Max: ulimit,
		Cur: ulimit,
	}
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

// TerminalWindowSize contains known bounds in rows, columns and pixels of the console
// behind the standard output.
type TerminalWindowSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// GetTerminalSize tries to retrieve information about the size of the console behind
// the standard output.
func GetTerminalSize() (TerminalWindowSize, error) {
	var ws TerminalWindowSize
	res, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	if res < 0 {
		return TerminalWindowSize{}, errno
	}
	return ws, nil
}
