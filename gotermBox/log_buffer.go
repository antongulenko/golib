package gotermBox

import (
	"container/ring"
	"fmt"
	"io"
	"sync"

	"github.com/antongulenko/golib"
	log "github.com/sirupsen/logrus"
)

// LogBuffer can be used to intercept the default logger of the "github.com/sirupsen/logrus" package
// and store all messages to a ring-buffer instead of outputting them directly.
type LogBuffer struct {
	// PushMessageHook is called each time a message is a added to this LogBuffer,
	// regardless if it was added from a logger or explicitly over PushMessage().
	PushMessageHook func(newMessage string)

	loggers           []*log.Logger
	originalLoggerOut []io.Writer

	messages       *ring.Ring
	msgLock        sync.Mutex
	message_buffer int
}

// NewDefaultLogBuffer creates a new LogBuffer of the given buffer size, that captures the logs
// of the default logger of the"github.com/sirupsen/logrus" package, and of the golib package.
func NewDefaultLogBuffer(message_buffer int) *LogBuffer {
	return NewLogBuffer(message_buffer, []*log.Logger{log.StandardLogger(), golib.Log})
}

// NewLogBuffer allocates a new LogBuffer instance with the given size for the message ring buffer.
func NewLogBuffer(message_buffer int, interceptedLoggers []*log.Logger) *LogBuffer {
	if message_buffer <= 0 || len(interceptedLoggers) == 0 {
		panic("message_buffer must be >0 and at least one logger to intercept must be given")
	}
	return &LogBuffer{
		messages:          ring.New(message_buffer),
		message_buffer:    message_buffer,
		loggers:           interceptedLoggers,
		originalLoggerOut: make([]io.Writer, len(interceptedLoggers)),
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
func (buf *LogBuffer) PrintMessages(w io.Writer, max_num int) (err error) {
	if max_num <= 0 {
		return
	}
	msgStart := buf.messages
	if max_num < buf.message_buffer {
		msgStart = msgStart.Move(-max_num)
	}
	msgStart.Do(func(msg interface{}) {
		if msg != nil && err == nil {
			_, err = fmt.Fprint(w, msg)
		}
	})
	return
}

// RegisterMessageHooks registers a hook for receiving log messages from all registered loggers.
// This should be called as early as possible in order to not miss any log messages.
// Any messages created prior to this will not be captured by the LogBuffer.
func (buf *LogBuffer) RegisterMessageHooks() {
	for _, l := range buf.loggers {
		l.Hooks.Add(buf)
	}
}

// InterceptLogger makes the registered loggers stop logging to their real output.
// The original logging output is stored, so it can be restored later with RestoreLogger().
func (buf *LogBuffer) InterceptLoggers() {
	for i, l := range buf.loggers {
		buf.originalLoggerOut[i] = l.Out
		l.Out = noopWriter{}
	}
}

// RestoreLogger restored the original logger output of the default logger of the
// "github.com/sirupsen/logrus" package. InterceptLogger() must have been called
// prior to this.
func (buf *LogBuffer) RestoreLoggers() {
	for i, l := range buf.loggers {
		l.Out = buf.originalLoggerOut[i]
	}
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

// Fire will be called with incoming log entries after RegisterMessageHooks() has
// been called. It uses the log formatter of the entry to format the message to a string
// and stores it using PushMessage().
func (buf *LogBuffer) Fire(entry *log.Entry) error {
	msg, err := entry.Logger.Formatter.Format(entry)
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
