package golib

import (
	"flag"
	"syscall"
	"unsafe"
)

var (
	ConfiguredOpenFilesLimit uint64
)

func RegisterOFLFlags() {
	flag.Uint64Var(&ConfiguredOpenFilesLimit, "ofl", ConfiguredOpenFilesLimit,
		"Set to >0 for configuring the open files limit (only possible as root)")
}

func ConfigureOpenFilesLimit() {
	if ConfiguredOpenFilesLimit > 0 {
		if err := SetOpenFilesLimit(ConfiguredOpenFilesLimit); err != nil {
			Log.Println("Failed to set open files limit to %v: %v", ConfiguredOpenFilesLimit, err)
		} else {
			Log.Println("Successfully set open files limit to %v", ConfiguredOpenFilesLimit)
		}
	}
}

func SetOpenFilesLimit(ulimit uint64) error {
	rLimit := syscall.Rlimit{
		Max: ulimit,
		Cur: ulimit,
	}
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
}

type TerminalWindowSize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

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
