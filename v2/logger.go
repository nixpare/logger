package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nixpare/broadcaster"
)

// Logger handles the logging. There are three types of Logger, depending on
// the way they save the logs: NewLogger() creates a Logger which saved every
// log in memory, NewHugeLogger() creates a Logger which keeps the most recent
// logs in memory and saves every logs in the filesystem, the method Clone() of
// every Logger creates a pseudo-Logger. You can read more in the package
// documentation for other info and usage senarios.
// For every Logger type, there is an io.Writer, which can be used to specify
// where to write the logs, and a list of tags, which will be given to every
// Log created with this Logger (in this way you can implement a filter to get
// only the logs you need).
// Logger also implements the io.Writer interface, so can be used to use it as
// a standard output: every write call corresponds to a Log with level
// logger.LOG_LEVEL_BLANK
type Logger interface {
	// AddLog can be used to manually create a Log, writeOutput can be set to false
	// if you also want the log to not be written to the io.Writer associated with
	// the Logger
	AddLog(level LogLevel, message string, extra string, writeOutput bool) int
	// Clone creates a pseudo-Logger that leans on the calling Logger, called the parent logger.
	// You can specify additional tags that will be inherited by every log created with
	// this logger, in addition to every tags owned by the parent logger. If you specify an out
	// io.Writer, every log will be also writter to this destination, and if you set parentOut to
	// true every log will be also written by the parent on its own output, possibly creating
	// multiple logs lines if the output is the same
	Clone(out io.Writer, parentOut bool, tags ...string) Logger
	// Debug is a shorthand for Print(logger.LOG_LEVEL_DEBUG, a...), can be handy while debugging
	// some code. See interface method Print for more information
	Debug(a ...any)
	// DisableExtras tells the Logger to not write the Extra field of the Logs created to the
	// out io.Writer (it will always be saved, so it can be accessed manually)
	DisableExtras()
	// EnableExtras tells the Logger to write the Extra field of the Logs created to the
	// out io.Writer. This is the default behaviour
	EnableExtras()
	// GetLastNLogs return the last Logs created by the Logger. If the number of logs asked is
	// greater than the total amount of logs created, the method will return every log saved
	// without errors
	GetLastNLogs(n int) []Log
	// GetLog returns a specific Log: being the "0" log the first created and the "nth-1" log the
	// nth created. Use the method NLogs() to know the total number of logs saved. It's not safe
	// to call it with an out-of-range index
	GetLog(index int) Log
	// GetLogs return every log in the interval [start ; end). It's not safe to call it with out-of-range
	// values
	GetLogs(start int, end int) []Log
	// GetSpecificLogs can be used to retrieve a list of logs. The argument holds the indexes of the
	// logs wanted
	GetSpecificLogs(logs []int) []Log
	// newLog creates a new log, tells wether it should be written to the out io.Writer and returns
	// the index of the newly log created for this specific Logger
	newLog(log Log, writeOutput bool) int
	// NLogs returns the number of logs saved in the logger
	NLogs() int
	// Out returns the same io.Writer provided at creation time
	Out() io.Writer
	// Print behaves has the fmt.Print or log.Print from the standard library, but if
	// the resulting output contains a line feed, everything after that will be used to
	// populate the extra field of the Log
	Print(level LogLevel, a ...any)
	// Printf behaves has the fmt.Printf or log.Printf from the standard library, but if
	// the resulting output contains a line feed, everything after that will be used to
	// populate the extra field of the Log
	Printf(level LogLevel, format string, a ...any)
	AsStdout() io.Writer
	AsStderr() io.Writer
	FixedLogger(level LogLevel) io.Writer
	Write(p []byte) (n int, err error)
	EnableHeavyLoadDetection()
	Close()
}

// DefaultLogger is the Logger used by the function in this package
// (like logger.Print, logger.Debug, ecc) and is initialized as a
// standard logger (logs are saved only in memory). Can be changed
// at any time and every process using this logger will reflect the
// change
var DefaultLogger Logger

var (
	// LogFileTimeFormat is the format that is used to create
	// the log files for the HugeLogger. It must not be changed
	// after the creation of the first HugeLogger, otherwise logs
	// with the old format will be lost
	LogFileTimeFormat = "06.01.02-15.04.05"
	// LogChunkSize determines both the numbers of logs kept in memory
	// and the number of logs saved in each file. It must not be changed
	// after the creation of the first HugeLogger
	LogChunkSize = 1000
	// LogFileExtension can be used to change the file extenstion of the
	// log files
	LogFileExtension = "data"

	MaxLogsPerScan           = 200
	ScanInterval             = 200 * time.Millisecond
	NegativeScansBeforeAlign = 5
)

// NewLogger creates a standard logger, which saves the logs only in
// memory. Read the Logger interface docs for other informations
func NewLogger(out io.Writer, tags ...string) Logger {
	return &memLogger{
		out:       out,
		v:         make([]Log, 0),
		tags:      tags,
		lastWrote: -1,
		rwm:       new(sync.RWMutex),
		alignM:    new(sync.Mutex),
		stopBc:    broadcaster.NewBroadcaster[struct{}](),
	}
}

// NewLogger creates a logger that keeps in memory the most recent logs and
// saves everything in files divided in clusters. The dir parameter tells the
// logger in which directory to save the logs' files. The prefix, instead, tells
// the logger how to name the files. Read the Logger interface docs for other informations
func NewHugeLogger(out io.Writer, dir string, prefix string, tags ...string) (*HugeLogger, error) {
	hls, err := initHugeLogStorage(dir, prefix)
	if err != nil {
		return nil, err
	}

	l := &HugeLogger{
		out:       out,
		hls:       hls,
		tags:      tags,
		lastWrote: -1,
		rwm:       new(sync.RWMutex),
		alignM:    new(sync.Mutex),
		stopBc:    broadcaster.NewBroadcaster[struct{}](),
	}

	return l, nil
}

func logToOut(l Logger, log Log, disableExtras bool) {
	out := l.Out()
	if level := log.Level(); out == os.Stdout && (level == LOG_LEVEL_WARNING || level == LOG_LEVEL_ERROR || level == LOG_LEVEL_FATAL) {
		out = os.Stderr
	}

	if ToTerminal(out) {
		if log.l.extra != "" && !disableExtras {
			fmt.Fprintln(out, log.l.fullColored())
		} else {
			fmt.Fprintln(out, log.l.colored())
		}
	} else {
		if log.l.extra != "" && !disableExtras {
			fmt.Fprintln(out, log.l.full())
		} else {
			fmt.Fprintln(out, log.l.String())
		}
	}
}

func asStdout(l Logger) io.Writer {
	return fixedLogger(l, log_level_stdout)
}

func asStderr(l Logger) io.Writer {
	return fixedLogger(l, log_level_stderr)
}

func fixedLogger(l Logger, level LogLevel) io.Writer {
	return &fixLogger{l: l, level: level}
}

func write(l Logger, p []byte) (n int, err error) {
	message := string(p)
	l.Print(LOG_LEVEL_BLANK, message)
	return len(message), nil
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

// Print is a shorthand for logger.DefaultLogger.Print, see Logger interface
// method description for any information
func Print(level LogLevel, a ...any) {
	DefaultLogger.Print(level, a...)
}

// Printf is a shorthand for logger.DefaultLogger.Printf, see Logger interface
// method description for any information
func Printf(level LogLevel, format string, a ...any) {
	DefaultLogger.Printf(level, format, a...)
}

// Debug is a shorthand for logger.DefaultLogger.Debug, see Logger interface
// method description for any information
func Debug(a ...any) {
	DefaultLogger.Debug(a...)
}
