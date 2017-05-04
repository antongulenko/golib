package golib

import (
	"sync"

	"github.com/nsf/termbox-go"
)

var (
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
	if err := termbox.Init(); err != nil {
		return ws, err
	}
	w, h := termbox.Size()
	ws.Col, ws.Row = uint16(w), uint16(h)
	termbox.Close()
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
