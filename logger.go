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
type Logger interface {
	Print(level LogLevel, message string, extra ...any)
	Printf(level LogLevel, format string, a ...any)
	Logs() []Log
	JSON() []byte
	Write(p []byte) (n int, err error)
	Out() io.Writer
	ChangeOut(out io.Writer)
}

type logger struct {
	out io.Writer
	logs []Log
}

func NewLogger(out io.Writer) Logger {
	return &logger {
		out: out,
		logs: make([]Log, 0),
	}
}

var default_logger Logger

func init() {
	default_logger = NewLogger(os.Stdout)
}

// Print creates a Log with the given severity and message; any data after message will be used
// to populate the extra field of the Log automatically using the built-in function
// fmt.Sprint(extra...)
func Print(level LogLevel, message string, extra ...any) {
	default_logger.Print(level, message, extra...)
}

// Debug is a shorthand for Print(LOG_LEVE_DEBUG, a...) used for debugging
func Debug(a ...any) {
	Print(LOG_LEVEL_DEBUG, fmt.Sprint(a...))
}

func (l *logger) Print(level LogLevel, message string, extra ...any) {
	log := NewLog(level, message, fmt.Sprint(extra...))
	l.logs = append(l.logs, log)

	if l.out != nil {
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
}

// Printf creates a Log with the given severity; the rest of the arguments is used as
// the built-in function fmt.Sprintf(format, a...), however if the resulting string
// contains a line feed, everything after that will be used to populate the extra field
// of the Log
func Printf(level LogLevel, format string, a ...any) {
	default_logger.Printf(level, format, a...)
}

// Debugf is a shorthand for Printf(LOG_LEVE_DEBUG, format, a...) used for debugging
func Debugf(format string, a ...any) {
	Printf(LOG_LEVEL_DEBUG, format, a...)
}

func (l *logger) Printf(level LogLevel, format string, a ...any) {
	str := fmt.Sprintf(format, a...)
	message, extra, _ := strings.Cut(str, "\n")
	l.Print(level, message, extra)
}

// Logs returns the list of logs stored
func (l *logger) Logs() []Log {
	logs := make([]Log, 0, len(l.logs))
	logs = append(logs, l.logs...)

	return logs
}

// JSON returns the list of logs stored in JSON format (see Log.JSON() method)
func (l *logger) JSON() []byte {
	b, err := json.Marshal(l.logs)
	if err != nil {
		panic(err)
	}

	return b
}

func (l *logger) Write(p []byte) (n int, err error) {
	message := string(p)
	l.Printf(LOG_LEVEL_BLANK, message)
	return len(message), nil
}

func (l *logger) Out() io.Writer {
	return l.out
}

func (l *logger) ChangeOut(out io.Writer) {
	l.out = out
}
