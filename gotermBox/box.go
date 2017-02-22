package gotermBox

import (
	"io"

	"github.com/antongulenko/goterm"
)

// CliLogBox can display a box spanning over the entire comand-line screen
// containing arbitrary content that can be updated from outside, and an additional
// section at the botton containing log messages captured using a LogBuffer.
type CliLogBox struct {
	// LogBuffer provides the underlying functionality for capturing log messages.
	// CliLogBox acts like an extension of LogBuffer. After calling Init(),
	// the log capturing must be taken care of using RegisterMessageHook(),
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
	box.LogBuffer = NewLogBuffer(box.MessageBuffer)
}

// Updates refreshes the entire display output. It can be called in arbitrary time intervals,
// but should never be called concurrently. The content must be written by the given function,
// which also receives the width of the screen. If it prints lines that are longer than the screen
// width, they will be cut off. It can produce an arbitrary number of lines.
func (self *CliLogBox) Update(writeContent func(out io.Writer, width int)) {
	box := goterm.NewBox(100|goterm.PCT, 100|goterm.PCT, 0)
	box.Height -= 1 // Line with cursor
	var separator, dots string
	if self.NoUtf8 {
		box.Border = "- | - - - -"
		separator = "-"
		dots = "... "
	} else {
		box.Border = "═ ║ ╔ ╗ ╚ ╝"
		separator = "═"
		dots = "··· "
	}
	lines := box.Height - 3 // borders + separator

	counter := newlineCounter{out: box}
	if self.LogLines > 0 {
		counter.max_lines = lines - self.LogLines
	}
	writeContent(&counter, box.Width)
	lines -= counter.num

	if counter.num > 0 {
		i := 0
		if counter.truncated {
			box.Write([]byte(dots))
			i += len(dots)
		}
		for i := 0; i < box.Width; i++ {
			box.Write([]byte(separator))
		}
		box.Write([]byte("\n"))
	}
	self.PrintMessages(box, lines)
	goterm.MoveCursor(1, 1)
	goterm.Print(box)
	goterm.Flush()
}

type newlineCounter struct {
	out       io.Writer
	num       int
	max_lines int
	truncated bool
}

func (counter *newlineCounter) Write(data []byte) (total int, err error) {
	total = len(data)
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
	return
}
