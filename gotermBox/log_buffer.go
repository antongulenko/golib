package gotermBox

import (
	"container/ring"
	"fmt"
	"io"
	"sync"

	log "github.com/sirupsen/logrus"
)

// LogBuffer can be used to intercept the default logger of the "github.com/sirupsen/logrus" package
// and store all messages to a ring-buffer instead of outputting them directly.
type LogBuffer struct {
	// PushMessageHook is called each time a message is a added to this LogBuffer,
	// regardless if it was added from a logger or explicitly over PushMessage().
	PushMessageHook func(newMessage string)

	messages          *ring.Ring
	msgLock           sync.Mutex
	message_buffer    int
	originalLoggerOut io.Writer
}

// NewLogBuffer allocates a new LogBuffer instance with the given size for the message ring buffer.
func NewLogBuffer(message_buffer int) *LogBuffer {
	if message_buffer <= 0 {
		panic("message_buffer must be >0")
	}
	return &LogBuffer{
		messages:       ring.New(message_buffer),
		message_buffer: message_buffer,
	}
}

// PushMessage adds a message to the message ring buffer.
func (buf *LogBuffer) PushMessage(msg string) {
	buf.msgLock.Lock()
	buf.messages.Value = msg
	buf.messages = buf.messages.Next()
	buf.msgLock.Unlock()
	if hook := buf.PushMessageHook; hook != nil {
		hook(msg)
	}
}

// PrintMessages prints all stored messages to the given io.Writer instance,
// optionally limiting the number of printed messages through the max_num parameter.
func (buf *LogBuffer) PrintMessages(w io.Writer, max_num int) {
	if max_num <= 0 {
		return
	}
	msgStart := buf.messages
	if max_num < buf.message_buffer {
		msgStart = msgStart.Move(-max_num)
	}
	msgStart.Do(func(msg interface{}) {
		if msg != nil {
			fmt.Fprint(w, msg)
		}
	})
}

// RegisterMessageHook registers a hook for receiving log messages from the default
// logger of the "github.com/sirupsen/logrus" package.
// This should be called as early as possible in order to not miss any log messages.
// Any messages created prior to this will not be captured by the LogBuffer.
func (buf *LogBuffer) RegisterMessageHook() {
	log.StandardLogger().Hooks.Add(buf)
}

// InterceptLogger makes the default logger of the "github.com/sirupsen/logrus" package
// stop logging to its real output. The original logging output is stored, so it
// can be restored later with RestoreLogger().
func (buf *LogBuffer) InterceptLogger() {
	buf.originalLoggerOut = log.StandardLogger().Out
	log.StandardLogger().Out = noopWriter{}
}

// RestoreLogger restored the original logger output of the default logger of the
// "github.com/sirupsen/logrus" package. InterceptLogger() must have been called
// prior to this.
func (buf *LogBuffer) RestoreLogger() {
	log.StandardLogger().Out = buf.originalLoggerOut
}

// Levels return all numbers in 0..255 to indicate that the LogBuffer will
// handle log messages of any level.
func (buf *LogBuffer) Levels() []log.Level {
	res := make([]log.Level, 256)
	for i := 0; i < len(res); i++ {
		res[int(i)] = log.Level(i)
	}
	return res
}

// Fire will be called with incoming log entries after RegisterMessageHook() has
// been called. It uses the default log formatter to format the message to a string
// and stores it using PushMessage().
func (buf *LogBuffer) Fire(entry *log.Entry) error {
	msg, err := log.StandardLogger().Formatter.Format(entry)
	if err != nil {
		return err
	}
	buf.PushMessage(string(msg))
	return nil
}

type noopWriter struct {
}

func (noopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
