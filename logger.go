package logger

import (
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
	parent      *Logger
	Out         io.Writer
	logs        []Log
	tags        []string
	wantExtras  bool
	multiLogger bool
}

func NewLogger(out io.Writer) *Logger {
	return &Logger{
		Out:  out,
		logs: make([]Log, 0),
		tags: make([]string, 0),
		wantExtras: true,
	}
}

var DefaultLogger *Logger

func (l *Logger) newLog(log Log, writeOutput bool) {
	log.addTags(l.tags...)

	if l.parent != nil {
		/* if !writeOutput {
			l.parent.newLog(log, writeOutput)
		} else {
			if l.Out == nil {
				l.parent.newLog(log, writeOutput)
			} else {
				if l.multiLogger && l.Out != l.parent.Out {
					l.parent.newLog(log, writeOutput)
				} else {
					l.parent.newLog(log, false)
				}
			}
		} */
		// Equivalent to the above
		if writeOutput && l.Out != nil && (!l.multiLogger || l.Out == l.parent.Out) {
			l.parent.newLog(log, false)
		} else {
			l.parent.newLog(log, writeOutput)
		}
	}

	l.logs = append(l.logs, log)

	if l.Out == nil || !writeOutput {
		return
	}

	out := l.Out
	if level := log.Level(); out == os.Stdout && (level == LOG_LEVEL_WARNING || level == LOG_LEVEL_ERROR || level == LOG_LEVEL_FATAL) {
		out = os.Stderr
	}

	if ToTerminal(l.Out) {
		if log.l.extra != "" && l.wantExtras {
			fmt.Fprintln(out, log.l.fullColored())
		} else {
			fmt.Fprintln(out, log.l.colored())
		}
	} else {
		if log.l.extra != "" && l.wantExtras {
			fmt.Fprintln(out, log.l.full())
		} else {
			fmt.Fprintln(out, log.l.String())
		}
	}
}

// AddLog appends a log without behing printed out
// on the Logger output or by any parent in cascade
func (l *Logger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *Logger) Print(level LogLevel, a ...any) {
	var str string
	first := true

	for _, x := range a {
		if first {
			first = false
		} else {
			str += " "
		}

		str += fmt.Sprint(x)
	}

	message, extra, _ := strings.Cut(str, "\n")
	l.AddLog(level, message, extra, true)
}

// Print creates a Log with the given severity and message; any data after message will be used
// to populate the extra field of the Log automatically using the built-in function
// fmt.Sprint(extra...)
func Print(level LogLevel, a ...any) {
	DefaultLogger.Print(level, a...)
}

func (l *Logger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

// Printf creates a Log with the given severity; the rest of the arguments is used as
// the built-in function fmt.Sprintf(format, a...), however if the resulting string
// contains a line feed, everything after that will be used to populate the extra field
// of the Log
func Printf(level LogLevel, format string, a ...any) {
	DefaultLogger.Printf(level, format, a...)
}

func (l *Logger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func Debug(a ...any) {
	DefaultLogger.Debug(a...)
}

// Logs returns the list of logs stored
func (l *Logger) Logs() []Log {
	logs := make([]Log, 0, len(l.logs))
	logs = append(logs, l.logs...)

	return logs
}

// JSON returns the list of logs stored in JSON format (see Log.JSON() method)
func (l *Logger) JSON() []byte {
	return LogsToJSON(l.logs)
}

// JSON returns the list of logs stored in JSON format (see Log.JSON() method)
func (l *Logger) JSONIndented(spaces int) []byte {
	return LogsToJSONIndented(l.logs, spaces)
}

func (l *Logger) Write(p []byte) (n int, err error) {
	message := string(p)
	l.Printf(LOG_LEVEL_BLANK, message)
	return len(message), nil
}

func (l *Logger) Clone(out io.Writer, tags ...string) *Logger {
	newLogger := NewLogger(out)

	newLogger.parent = l
	newLogger.AddTags(tags...)

	return newLogger
}

func (l *Logger) AddTags(tags ...string) {
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		tag = strings.ToLower(tag)
		for _, lTags := range l.tags {
			if tag == lTags {
				continue
			}
		}
		
		l.tags = append(l.tags, tag)
	}
}

func (l *Logger) EnableExtras() {
	l.wantExtras = true
}

func (l *Logger) DisableExtras() {
	l.wantExtras = false
}

func (l *Logger) SetParent(parent *Logger) {
	l.parent = parent
}

func (l *Logger) EnableMultiLogger() {
	l.multiLogger = true
}

func (l *Logger) DisableMultiLogger() {
	l.multiLogger = false
}

func (l *Logger) LogsMatch(tags ...string) []Log {
	lMatch := make([]Log, 0)
	for _, log := range l.logs {
		if log.Match(tags...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}

func (l *Logger) LogsMatchAny(tags ...string) []Log {
	lMatch := make([]Log, 0)
	for _, log := range l.logs {
		if log.MatchAny(tags...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}

func (l *Logger) LogsLevelMatchAny(levels ...LogLevel) []Log {
	lMatch := make([]Log, 0)
	for _, log := range l.logs {
		if log.LevelMatchAny(levels...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}
