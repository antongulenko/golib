package gotermBox

import (
	"io"

	"github.com/antongulenko/golib"
	"github.com/antongulenko/goterm"
)

// CliLogBox can display a box spanning over the entire command-line screen
// containing arbitrary content that can be updated from outside, and an additional
// section at the bottom containing log messages captured using a LogBuffer.
type CliLogBox struct {
	// LogBuffer provides the underlying functionality for capturing log messages.
	// CliLogBox acts like an extension of LogBuffer. After calling Init(),
	// the log capturing must be taken care of using RegisterMessageHooks(),
	// InterceptLogger() and RestoreLogger().
	*LogBuffer

	// NoUtf8 can be set to true to replace UTF8 border drawing characters with
	// regular ASCII characters.
	NoUtf8 bool

	// LogLines configures the minimum number of log entries that must remain visible
	// in the lower part of the console box. The log entries are appended directly
	// to the actual output and usually take the rest of the screen. If the box content
	// is so long, that it would leave less than LogLines log entries, the content is
	// truncated. The truncation is visualized by three dots in the separator line
	// between the content and the log entries.
	LogLines int

	// MessageBuffer configures the number of messages stored in the underlying LogBuffer.
	MessageBuffer int
}

// Init initializes the underlying LogBuffer and should be called before any other methods.
func (box *CliLogBox) Init() {
	box.LogBuffer = NewDefaultLogBuffer(box.MessageBuffer)
}

// Updates refreshes the entire display output. It can be called in arbitrary time intervals,
// but should never be called concurrently. The content must be written by the given function,
// which also receives the width of the screen. If it prints lines that are longer than the screen
// width, they will be cut off. It can produce an arbitrary number of lines.
func (box *CliLogBox) Update(writeContent func(out io.Writer, width int)) {
	console := golib.GetTerminalSize()
	gotermBox := goterm.NewBox(int(console.Col), int(console.Row), 0)

	gotermBox.Height -= 1 // Line with cursor
	var separator, dots string
	if box.NoUtf8 {
		gotermBox.Border = "- | - - - -"
		separator = "-"
		dots = "... "
	} else {
		gotermBox.Border = "═ ║ ╔ ╗ ╚ ╝"
		separator = "═"
		dots = "··· "
	}
	lines := gotermBox.Height - 3 // borders + separator

	counter := newlineCounter{out: gotermBox}
	if box.LogLines > 0 {
		counter.max_lines = lines - box.LogLines
	}
	writeContent(&counter, gotermBox.Width)
	lines -= counter.num

	if counter.num > 0 {
		i := 0
		if counter.truncated {
			gotermBox.Write([]byte(dots))
			i += len(dots)
		}
		for i := 0; i < gotermBox.Width; i++ {
			gotermBox.Write([]byte(separator))
		}
		gotermBox.Write([]byte("\n"))
	}
	box.PrintMessages(gotermBox, lines)
	goterm.MoveCursor(1, 1)
	goterm.Print(gotermBox)
	goterm.Flush()
}

type newlineCounter struct {
	out       io.Writer
	num       int
	max_lines int
	truncated bool
}

func (counter *newlineCounter) Write(data []byte) (int, error) {
	total := len(data)
	if counter.max_lines <= 0 || counter.num < counter.max_lines {
		written := 0
		start := 0
		for i, char := range data {
			if char == '\n' {
				counter.num++
				num, err := counter.out.Write(data[start : i+1])
				written += num
				start = i + 1
				if err != nil {
					return written, err
				}
				if counter.max_lines > 0 && counter.num >= counter.max_lines {
					return total, err
				}
			}
		}
		num, err := counter.out.Write(data[start:])
		written += num
		if err != nil {
			return written, err
		}
	} else {
		counter.truncated = true
	}
	return total, nil
}
