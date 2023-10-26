package logger

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nixpare/comms"
)

type memLogger struct {
	out            io.Writer
	v              []Log
	tags           []string
	extrasDisabled bool
	counter        int
	heavyLoad      bool
	lastWrote      int
	rwm            *sync.RWMutex
	alignM         *sync.Mutex
	stopBc         *comms.Broadcaster[struct{}]
}

func (l *memLogger) newLog(log Log, writeOutput bool) int {
	l.counter++
	log.addTags(l.tags...)

	l.rwm.Lock()

	l.v = append(l.v, log)
	p := len(l.v) - 1

	if l.out == nil || !writeOutput {
		l.lastWrote = p
		l.rwm.Unlock()
		return p
	}

	if !l.heavyLoad && l.lastWrote == p-1 {
		l.lastWrote = p
		l.rwm.Unlock()

		logToOut(l, log, l.extrasDisabled)
	} else {
		l.rwm.Unlock()
	}

	return p
}

func (l *memLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *memLogger) Print(level LogLevel, a ...any) {
	print(l, level, a...)
}

func (l *memLogger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

func (l *memLogger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func (l *memLogger) NLogs() int {
	return len(l.v)
}

func (l *memLogger) Out() io.Writer {
	return l.out
}

func (l *memLogger) GetLog(index int) Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.v[index]
}

func (l *memLogger) GetLastNLogs(n int) []Log {
	tot := l.NLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *memLogger) GetLogs(start, end int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.v[start:end]
}

func (l *memLogger) GetSpecificLogs(logs []int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	res := make([]Log, 0, len(logs))
	for _, p := range logs {
		res = append(res, l.v[p])
	}
	return res
}

func (l *memLogger) AsStdout() io.Writer {
	return asStdout(l)
}

func (l *memLogger) AsStderr() io.Writer {
	return asStderr(l)
}

func (l *memLogger) FixedLogger(level LogLevel) io.Writer {
	return fixedLogger(l, level)
}

func (l *memLogger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *memLogger) EnableExtras() {
	l.extrasDisabled = false
}

func (l *memLogger) DisableExtras() {
	l.extrasDisabled = true
}

func (l *memLogger) Clone(out io.Writer, parentOut bool, tags ...string) Logger {
	return newCloneLogger(l, out, parentOut, tags, l.extrasDisabled)
}

func (l *memLogger) checkHeavyLoad() {
	ticker := time.NewTicker(ScanInterval)
	var exitLoop bool

	stopC := make(chan struct{})
	defer close(stopC)

	var stopMsg comms.BroadcastMessage[struct{}]
	go func() {
		stopMsg = l.stopBc.Listen()
		stopC <- struct{}{}
	}()

	var doingPartialAlign bool

	for !exitLoop {
		select {
		case <-ticker.C:
			if l.counter > MaxLogsPerScan {
				l.heavyLoad = true
			} else {
				l.heavyLoad = false

				if !doingPartialAlign {
					doingPartialAlign = true
					go func() {
						l.alignOutput(false)
						doingPartialAlign = false
					}()
				}
			}

			l.counter = 0
		case <-stopC:
			ticker.Stop()
			exitLoop = true

			l.alignOutput(true)
		}
	}

	stopMsg.Report()
}

func (l *memLogger) EnableHeavyLoadDetection() {
	if l.out != nil {
		go l.checkHeavyLoad()
	}
}

func (l *memLogger) Close() {
	l.stopBc.SendAndWait(struct{}{})
}

func (l *memLogger) alignOutput(empty bool) {
	l.alignM.Lock()
	defer l.alignM.Unlock()

	for {
		if !empty && l.heavyLoad {
			break
		}

		if l.lastWrote == -1 {
			break
		}

		logs := l.GetLastNLogs(l.NLogs() - l.lastWrote - 1)

		if len(logs) == 0 {
			break
		}

		if len(logs) > MaxLogsPerScan {
			logs = logs[:MaxLogsPerScan]
		}

		for _, log := range logs {
			logToOut(l, log, l.extrasDisabled)
		}

		l.rwm.Lock()
		l.lastWrote += len(logs)
		l.rwm.Unlock()
	}
}
