package golib

import (
	"flag"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

var (
	// ConfiguredOpenFilesLimit is used by ConfigureOpenFilesLimit() to try to configure allowed the number
	// of open files for the current process. It is set by the '-ofl' flag created by the RegisterOFLFlags()
	// function.
	ConfiguredOpenFilesLimit uint64

	// DefaultTerminalWindowSize is used as a default result by GetTerminalSize() if the real
	// terminal size cannot be determined.
	DefaultTerminalWindowSize = TerminalWindowSize{
		Row:    50,
		Col:    160,
		Xpixel: 0,
		Ypixel: 0,
	}

	warnTerminalSizeOnce sync.Once
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
func GetTerminalSizeErr() (TerminalWindowSize, error) {
	var ws TerminalWindowSize
	res, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	if res < 0 || errno != 0 {
		return TerminalWindowSize{}, os.NewSyscallError("GetWinsize", errno)
	}
	return ws, nil
}

// GetTerminalSize tries to retrieve information about the size of the console behind
// the standard output. If the query fails, it prints a warning to the logger and
// returns the default value DefaultTerminalWindowSize.
func GetTerminalSize() TerminalWindowSize {
	res, err := GetTerminalSizeErr()
	if err != nil {
		res = DefaultTerminalWindowSize
		warnTerminalSizeOnce.Do(func() {
			Log.Warnf("Failed to get terminal size: %v. Using default: %+v", err, DefaultTerminalWindowSize)
		})
	}
	return res
}
