package golib

import (
	"bytes"
	"flag"
	"time"

	"github.com/chris-garrett/lfshook"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var (
	// LogFile can be set to non-empty to make ConfigureLogging output all log
	// entries in addition to showing them on the standard error stream.
	// All entries are output to the file, regardless of the log-level configured for the
	// console output.
	LogFile string

	// LogVerbose makes the ConfigureLogging() function set the global log level to Debug.
	LogVerbose bool

	// LogQuiet makes the ConfigureLogging() function set the global log level to Warn, but
	// only of LogVerbose is not set.
	LogQuiet bool

	// LogVeryQuiet makes the ConfigureLogging() function set the global log level to Error,
	// but only if LogVerbose and LogQuiet are both not set.
	LogVeryQuiet bool

	// Log is the package-wide logger for the golib package. It can be configured or disabled.
	Log = log.New()
)

func init() {
	formatter := newLogFormatter()
	log.StandardLogger().SetFormatter(formatter)
	Log.SetFormatter(formatter)
}

// RegisterLogFlags registers flags for changing variables that will control
// the log level and other logging parameters when calling ConfigureLogging().
func RegisterLogFlags() {
	flag.BoolVar(&LogVerbose, "v", false, "Enable verbose logging output")
	flag.BoolVar(&LogQuiet, "q", false, "Suppress logging output (except warnings and errors)")
	flag.BoolVar(&LogVeryQuiet, "qq", false, "Suppress logging output (except errors)")
	flag.StringVar(&LogFile, "log", "", "Redirect logs to a given file in addition to the console.")
}

// ConfigureLogging configures the logger based on the global Log* variables defined in the package.
// It calls ConfigureLogger() for the standard Logrus logger and the logger of this package.
// This function should be called early in every main() function, preferably before any prior logging output,
// but after calling RegisterLogFlags() and flag.Parse().
func ConfigureLogging() {
	ConfigureLogger(Log)
	ConfigureLogger(log.StandardLogger())
}

// ConfigureLogger configures the given logger based on Log* variables defined in the package.
func ConfigureLogger(l *log.Logger) {
	level := log.InfoLevel
	if LogVerbose {
		level = log.DebugLevel
	} else if LogVeryQuiet {
		level = log.ErrorLevel
	} else if LogQuiet {
		level = log.WarnLevel
	}
	l.SetLevel(level)
	if LogFile != "" {
		pathmap := make(lfshook.PathMap)
		for i := 0; i < 256; i++ {
			pathmap[log.Level(i)] = LogFile
		}
		hook := lfshook.NewHook(pathmap)
		hook.SetFormatter(newLogFormatter())
		l.AddHook(hook)
	}
}

func newLogFormatter() *myFormatter {
	return &myFormatter{
		f: log.TextFormatter{
			DisableColors:   false,
			ForceColors:     true,
			FullTimestamp:   true,
			TimestampFormat: time.StampMilli,
		},
	}
}

type myFormatter struct {
	f logrus.TextFormatter
}

func (f *myFormatter) Format(e *logrus.Entry) ([]byte, error) {
	text, err := f.f.Format(e)
	if err != nil {
		return text, err
	}
	// Remove all whitespace and replace with a single trailing newline character
	// Many libraries explicitly add a \n character to log lines, which leads to empty lines.
	text = bytes.TrimSpace(text)
	text = append(text, '\n')
	return text, nil
}
