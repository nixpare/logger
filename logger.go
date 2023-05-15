package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Logger is used by the Router and can be used by the user to
// create logs that are both written to the chosen io.Writer (if any)
// and saved locally in memory, so that they can be retreived
// programmatically and used (for example to make a view in a website)
type Logger struct {
	parent *Logger
	out    io.Writer
	logs   []Log
	tags   []string
}

func NewLogger(out io.Writer) *Logger {
	return &Logger {
		out: out,
		logs: make([]Log, 0),
		tags: nil,
	}
}

var default_logger *Logger

func DefaultLogger() *Logger {
	return default_logger
}

func init() {
	default_logger = NewLogger(os.Stdout)
}

func (l *Logger) addLog(log Log) {
	if l.parent != nil {
		l.parent.addLog(log)
	}

	l.logs = append(l.logs, log)
	if l.out == nil {
		return
	}

	if ToTerminal(l.out) {
		if log.Extra != "" {
			fmt.Fprintf(l.out, "%v\n%s\n", log.Colored(), IndentString(log.Extra, 4))
		} else {
			fmt.Fprintln(l.out, log.Colored())
		}
	} else {
		if log.Extra != "" {
			fmt.Fprintf(l.out, "%v\n%s\n", l, IndentString(log.Extra, 4))
		} else {
			fmt.Fprintln(l.out, l)
		}
	}
}

// Appends only on the calling logger, on the parent cascade
// it's also printed out
func (l *Logger) AppendLog(log Log) {
	if l.parent != nil {
		l.parent.addLog(log)
	}
	l.logs = append(l.logs, log)
}

func (l *Logger) Print(level LogLevel, a ...any) {
	str := fmt.Sprint(a...)
	message, extra, _ := strings.Cut(str, "\n")

	log := NewLog(level, message, extra, l.tags...)
	l.addLog(log)
}

// Print creates a Log with the given severity and message; any data after message will be used
// to populate the extra field of the Log automatically using the built-in function
// fmt.Sprint(extra...)
func Print(level LogLevel, a ...any) {
	default_logger.Print(level, a...)
}

func (l *Logger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

// Printf creates a Log with the given severity; the rest of the arguments is used as
// the built-in function fmt.Sprintf(format, a...), however if the resulting string
// contains a line feed, everything after that will be used to populate the extra field
// of the Log
func Printf(level LogLevel, format string, a ...any) {
	default_logger.Printf(level, format, a...)
}

func (l *Logger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func Debug(a ...any) {
	default_logger.Debug(a...)
}

// Logs returns the list of logs stored
func (l *Logger) Logs() []Log {
	logs := make([]Log, 0, len(l.logs))
	logs = append(logs, l.logs...)

	return logs
}

// JSON returns the list of logs stored in JSON format (see Log.JSON() method)
func (l *Logger) JSON() []byte {
	b, err := json.Marshal(l.Logs())
	if err != nil {
		panic(err)
	}

	return b
}

// JSON returns the list of logs stored in JSON format (see Log.JSON() method)
func (l *Logger) JSONIndented(spaces int) []byte {
	indent := ""
	for i := 0; i < spaces; i++ {
		indent += " "
	}

	b, err := json.MarshalIndent(l.Logs(), "", indent)
	if err != nil {
		panic(err)
	}

	return b
}

func (l *Logger) Write(p []byte) (n int, err error) {
	message := string(p)
	l.Printf(LOG_LEVEL_BLANK, message)
	return len(message), nil
}

func (l *Logger) Out() io.Writer {
	return l.out
}

func (l *Logger) Clone(out io.Writer, tags ...string) *Logger {
	logger := NewLogger(out)
	logger.parent = l
	logger.tags = append(l.tags, tags...)

	return logger
}

func Clone(out io.Writer, tags ...string) *Logger {
	logger := NewLogger(out)
	logger.parent = default_logger
	logger.tags = append(default_logger.tags, tags...)

	return logger
}
