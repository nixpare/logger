package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Logger is used by the Router and can be used by the user to
// create logs that are both written to the chosen io.Writer (if any)
// and saved locally in memory, so that they can be retreived
// programmatically and used (for example to make a view in a website)
type Logger interface {
	AddLog(level LogLevel, message string, extra string, writeOutput bool)
	Clone(out io.Writer, tags ...string) Logger
	Debug(a ...any)
	DisableExtras()
	EnableExtras()
	GetLastNLogs(n int) []Log
	GetLog(index int) Log
	GetLogs(start int, end int) []Log
	newLog(log Log, writeOutput bool) int
	NLogs() int
	Out() io.Writer
	Print(level LogLevel, a ...any)
	Printf(level LogLevel, format string, a ...any)
	Write(p []byte) (n int, err error)
}

type logger struct {
	out         io.Writer
	logs        logStorage
	tags        []string
	disableExtras  bool
}

var DefaultLogger Logger

func NewLogger(out io.Writer, tags ...string) Logger {
	return &logger{
		out: out,
		logs: &memLogStorage{
			v:   make([]Log, 0),
			rwm: new(sync.RWMutex),
		},
		tags:       tags,
	}
}

func NewHugeLogger(out io.Writer, dir string, prefix string, tags ...string) (Logger, error) {
	fls, err := initFileLogStorage(dir, prefix)
	if err != nil {
		return nil, err
	}

	return &logger{
		out:  out,
		logs: fls,
		tags: tags,
	}, nil
}

func (l *logger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)
	p := l.logs.addLog(log)

	if l.out == nil || !writeOutput {
		return p
	}

	out := l.out
	if level := log.Level(); out == os.Stdout && (level == LOG_LEVEL_WARNING || level == LOG_LEVEL_ERROR || level == LOG_LEVEL_FATAL) {
		out = os.Stderr
	}

	if ToTerminal(l.out) {
		if log.l.extra != "" && !l.disableExtras {
			fmt.Fprintln(out, log.l.fullColored())
		} else {
			fmt.Fprintln(out, log.l.colored())
		}
	} else {
		if log.l.extra != "" && !l.disableExtras {
			fmt.Fprintln(out, log.l.full())
		} else {
			fmt.Fprintln(out, log.l.String())
		}
	}

	return p
}

// AddLog appends a log without behing printed out
// on the Logger output or by any parent in cascade
func (l *logger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func print(l Logger, level LogLevel, a ...any) {
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

func (l *logger) Print(level LogLevel, a ...any) {
	print(l, level, a)
}

// Print creates a Log with the given severity and message; any data after message will be used
// to populate the extra field of the Log automatically using the built-in function
// fmt.Sprint(extra...)
func Print(level LogLevel, a ...any) {
	DefaultLogger.Print(level, a...)
}

func (l *logger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

// Printf creates a Log with the given severity; the rest of the arguments is used as
// the built-in function fmt.Sprintf(format, a...), however if the resulting string
// contains a line feed, everything after that will be used to populate the extra field
// of the Log
func Printf(level LogLevel, format string, a ...any) {
	DefaultLogger.Printf(level, format, a...)
}

func (l *logger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func Debug(a ...any) {
	DefaultLogger.Debug(a...)
}

func (l *logger) NLogs() int {
	return l.logs.nLogs()
}

func (l *logger) Out() io.Writer {
	return l.out
}

func (l *logger) GetLog(index int) Log {
	return l.logs.getLog(index)
}

func (l *logger) GetLastNLogs(n int) []Log {
	tot := l.logs.nLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *logger) GetLogs(start, end int) []Log {
	return l.logs.getLogs(start, end)
}

func write(l Logger, p []byte) (n int, err error) {
	message := string(p)
	l.Print(LOG_LEVEL_BLANK, message)
	return len(message), nil
}

func (l *logger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *logger) EnableExtras() {
	l.disableExtras = false
}

func (l *logger) DisableExtras() {
	l.disableExtras = true
}

func (l *logger) Clone(out io.Writer, tags ...string) Logger {
	return &cloneLogger{
		out:        out,
		tags:       tags,
		disableExtras: l.disableExtras,
		parent:     l,
	}
}
