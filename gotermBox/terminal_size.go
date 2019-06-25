package gotermBox

import (
	"fmt"
	"sync"

	"github.com/antongulenko/goterm"
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
// the standard output. If the query fails, it prints a warning to the logger and
// returns the default value DefaultTerminalWindowSize.
func GetTerminalSize() TerminalWindowSize {
	ws := DefaultTerminalWindowSize
	w, h := goterm.Width(), goterm.Height()
	if w >= 0 {
		ws.Col = uint16(w)
	}
	if h >= 0 {
		ws.Row = uint16(h)
	}
	if w < 0 || h < 0 {
		warnTerminalSizeOnce.Do(func() {
			fmt.Printf("Failed to get terminal size, using default: %+v", DefaultTerminalWindowSize)
		})
	}
	return ws
}
