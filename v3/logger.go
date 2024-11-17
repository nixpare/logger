package logger

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nixpare/broadcaster"
)

var (
	// LogFileTimeFormat is the format that is used to create
	// the log files for the HugeLogger. It must not be changed
	// after the creation of the first HugeLogger, otherwise logs
	// with the old format will be lost
	LogFileTimeFormat = "06.01.02-15.04.05"
	// LogChunkSize determines both the numbers of logs kept in memory
	// and the number of logs saved in each file. It must not be changed
	// after the creation of the first HugeLogger.
	// The default value (100_000) is a good compromise between memory
	// usage and operations speed, considering that each chunk of logs
	// takes 5MB of memory with this value
	LogChunkSize = 100_000
	// LogFileExtension can be used to change the file extenstion of the
	// log files
	LogFileExtension = "data"
	MaxLogsPerScan           = 200
	ScanInterval             = 200 * time.Millisecond
	NegativeScansBeforeAlign = 5
	AlignChunkSize           = 1_000
	MaxMemUsage uint64       = 1_000_000_000
)

type Logger struct {
	out            io.Writer
	storage        storage
	tags           []string
	extrasDisabled bool
	counter        int
	heavyLoad      bool
	nextToWrite    int

	logBroadcaster *broadcaster.Broadcaster[Log]

	rwm    *sync.RWMutex
	alignM *sync.Mutex
	stopBc *broadcaster.BroadcastWaiter[struct{}]
}

// DefaultLogger is the Logger used by the function in this package
// (like logger.Print, logger.Debug, ecc) and is initialized as a
// standard logger (logs are saved only in memory). Can be changed
// at any time and every process using this logger will reflect the
// change
var DefaultLogger *Logger

func newLogger(out io.Writer, tags ...string) *Logger {
	return &Logger{
		out:         out,
		tags:        tags,

		logBroadcaster: broadcaster.NewBroadcaster[Log](),

		rwm:         new(sync.RWMutex),
		alignM:      new(sync.Mutex),
		stopBc:      broadcaster.NewBroadcastWaiter[struct{}](),
	}
}

// NewLogger creates a standard logger, which saves the logs only in
// memory. Read the Logger interface docs for other informations
func NewLogger(out io.Writer, tags ...string) *Logger {
	l := newLogger(out, tags...)
	l.storage = &memLogStorage{}
	return l
}

// NewLogger creates a logger that keeps in memory the most recent logs and
// saves everything in files divided in clusters. The dir parameter tells the
// logger in which directory to save the logs' files. The prefix, instead, tells
// the logger how to name the files. Read the Logger interface docs for other informations
func NewHugeLogger(out io.Writer, dir string, prefix string, tags ...string) (*Logger, error) {
	hls, err := initHugeLogStorage(dir, prefix)
	if err != nil {
		return nil, err
	}

	l := newLogger(out, tags...)
	l.storage = hls

	return l, nil
}

func (l *Logger) Clone(out io.Writer, parentOut bool, tags ...string) *Logger {
	clone := newLogger(out, tags...)
	clone.storage = &cloneLogStorage{
		parent: l,
		parentOut: parentOut,
	}
	return clone
}

func (l *Logger) Close() {
	l.logBroadcaster.Close()
	l.stopBc.Close()
}

func (l *Logger) newLog(log Log, writeOutput bool) int {
	log.avoidLogging = !writeOutput

	l.counter++
	log.addTags(l.tags...)

	l.rwm.Lock()

	l.storage.addLog(log)
	l.logBroadcaster.Send(log)
	p := l.storage.logs() - 1

	if l.out == nil {
		l.nextToWrite++
		l.rwm.Unlock()
		return p
	}

	if !l.heavyLoad && l.nextToWrite == p {
		l.nextToWrite++
		l.rwm.Unlock()

		l.logToOut(log, l.extrasDisabled)
	} else {
		l.rwm.Unlock()
	}

	return p
}

func (l *Logger) AddLog(level LogLevel, message string, extra string, writeOutput bool) int {
	t := time.Now()

	return l.newLog(Log{
		l: &log{
			id: fmt.Sprintf(
				"%d%03d",
				t.UnixNano() / 1000, rand.Intn(1000),
			),
			level: level, date: t,
			message: strings.TrimSpace(message), extra: strings.TrimSpace(extra),
		},
	}, writeOutput)
}

func (l *Logger) logToOut(log Log, disableExtras bool) {
	if log.avoidLogging {
		return
	}

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

// Print is a shorthand for logger.DefaultLogger.Print, see Logger interface
// method description for any information
func Print(level LogLevel, a ...any) {
	DefaultLogger.Print(level, a...)
}

func (l *Logger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

// Printf is a shorthand for logger.DefaultLogger.Printf, see Logger interface
// method description for any information
func Printf(level LogLevel, format string, a ...any) {
	DefaultLogger.Printf(level, format, a...)
}

func (l *Logger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

// Debug is a shorthand for logger.DefaultLogger.Debug, see Logger interface
// method description for any information
func Debug(a ...any) {
	DefaultLogger.Debug(a...)
}

func (l *Logger) Logs() int {
	return l.storage.logs()
}

func (l *Logger) Out() io.Writer {
	return l.out
}

func (l *Logger) GetLog(index int) Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.storage.getLog(index)
}

func (l *Logger) GetLastNLogs(n int) []Log {
	tot := l.Logs()
	if n > tot {
		n = tot
	}
	
	return l.GetLogs(tot-n, tot)
}

func (l *Logger) GetLogs(start, end int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	if end > l.storage.logs() {
		end = l.storage.logs()
	}

	return l.storage.getLogs(start, end)
}

func (l *Logger) GetSpecificLogs(logs []int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.storage.getSpecificLogs(logs)
}

func (l *Logger) ListenForLogs(bufSize int) (int, *broadcaster.Channel[Log]) {
	l.rwm.RLock()
	defer l.rwm.RUnlock()
	
	return l.Logs(), l.logBroadcaster.Register(bufSize)
}

func (l *Logger) AsStdout() io.Writer {
	return l.FixedLogger(log_level_stdout)
}

func (l *Logger) AsStderr() io.Writer {
	return l.FixedLogger(log_level_stderr)
}

func (l *Logger) FixedLogger(level LogLevel) io.Writer {
	return &fixLogger{l: l, level: level}
}

func (l *Logger) Write(p []byte) (n int, err error) {
	message := string(p)
	l.Print(LOG_LEVEL_BLANK, message)
	return len(message), nil
}

func (l *Logger) EnableExtras() {
	l.extrasDisabled = false
}

func (l *Logger) DisableExtras() {
	l.extrasDisabled = true
}

func memUsageExceeded() bool {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem.Alloc > MaxMemUsage
}

func (l *Logger) checkHeavyLoad() {
	ticker := time.NewTicker(ScanInterval)
	var exitLoop bool

	stopC := make(chan struct{})
	defer close(stopC)

	var stopMsg broadcaster.Payload[struct{}]
	listener := l.stopBc.Register(1)
	defer listener.Unregister()

	go func() {
		stopMsg = <-listener.Ch()
		stopC <- struct{}{}
	}()

	var alignInProgress bool
	var releaseCounter int

	l.heavyLoad = true
	l.storage.enableEavyLoad()

loop:
	for !exitLoop {
		select {
		case <-ticker.C:
			if l.storage.needsFlushing() {
				ticker.Stop()
				l.storage.align(true)
				ticker.Reset(ScanInterval)
				continue loop
			}

			if l.counter > MaxLogsPerScan {
				releaseCounter = 0
			} else {
				releaseCounter++

				if releaseCounter > NegativeScansBeforeAlign && !alignInProgress {
					alignInProgress = true
					releaseCounter = 0

					go func() {
						l.alignOutput(false)
						l.storage.align(false)

						alignInProgress = false
					}()
				}
			}

			l.counter = 0
		case <-stopC:
			ticker.Stop()
			exitLoop = true
		}
	}

	l.storage.disableEavyLoad()
	l.heavyLoad = false

	l.alignOutput(true)
	l.storage.align(true)

	stopMsg.Done()
}

func (l *Logger) EnableHeavyLoad() {
	if l.out != nil {
		go l.checkHeavyLoad()
	}
}

func (l *Logger) DisableHeavyLoad() {
	l.stopBc.Send(struct{}{}).Wait()
}

func (l *Logger) alignOutput(empty bool) {
	l.alignM.Lock()
	defer l.alignM.Unlock()

	if l.Logs() == 0 {
		return
	}

	if !empty {
		logs := l.GetLogs(l.nextToWrite, l.nextToWrite+AlignChunkSize)
		l.rwm.Lock()
		l.nextToWrite += len(logs)
		l.rwm.Unlock()

		for _, log := range logs {
			l.logToOut(log, l.extrasDisabled)
		}

		return
	}

	for logs := range l.GetLogsBuffered(l.nextToWrite, l.Logs()) {
		if len(logs) == 0 {
			break
		}

		l.rwm.Lock()
		l.nextToWrite += len(logs)
		l.rwm.Unlock()

		for _, log := range logs {
			l.logToOut(log, l.extrasDisabled)
		}
	}
}

func (l *Logger) GetLastNLogsBuffered(n int) <-chan []Log {
	tot := l.Logs()
	if n > tot {
		n = tot
	}
	return l.GetLogsBuffered(tot-n, tot)
}

func (l *Logger) GetLogsBuffered(start, end int) <-chan []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	c := make(chan []Log)

	go func() {
		defer close(c)

		var i int
		for i = start; i+AlignChunkSize < end; i += AlignChunkSize {
			c <- l.storage.getLogs(i, i+AlignChunkSize)
		}
		if i < end {
			c <- l.storage.getLogs(i, end)
		}
	}()

	return c
}

type fixLogger struct {
	l     *Logger
	level LogLevel
}

func (fl *fixLogger) Write(p []byte) (n int, err error) {
	fl.l.Print(fl.level, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
