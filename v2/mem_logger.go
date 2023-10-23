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
	rwm *sync.RWMutex
	tags           []string
	disableExtras  bool
	counter        int
	heavyLoad      bool
	lastWrote      int
	stopBc         *comms.Broadcaster[struct{}]
}

func newMemLogger(out io.Writer, tags []string) *memLogger {
	return &memLogger{
		out:     out,
		v: make([]Log, 0),
		rwm: new(sync.RWMutex),
		tags:    tags,
		stopBc:   comms.NewBroadcaster[struct{}](),
	}
}

func (l *memLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)

	l.rwm.Lock()
	l.v = append(l.v, log)
	p := len(l.v)-1
	l.rwm.Unlock()

	if l.out == nil || !writeOutput {
		return p
	}

	if !l.heavyLoad {
		l.lastWrote = p
		logToOut(l, log, l.disableExtras)
		return p
	}

	return p
}

func (l *memLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.counter++

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

func (l *memLogger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *memLogger) EnableExtras() {
	l.disableExtras = false
}

func (l *memLogger) DisableExtras() {
	l.disableExtras = true
}

func (l *memLogger) Clone(out io.Writer, tags ...string) Logger {
	return &cloneLogger{
		out:           out,
		tags:          tags,
		disableExtras: l.disableExtras,
		parent:        l,
	}
}

func (l *memLogger) checkHeavyLoad() {
	ticker := time.NewTicker(time.Second)
	var exitLoop bool
	
	stopC := make(chan struct{})
	defer close(stopC)

	var stopMsg comms.BroadcastMessage[struct{}]
	go func() {
		stopMsg = l.stopBc.Listen()
		stopC <- struct{}{}
	}()
	
	for !exitLoop {
		select {
		case <- ticker.C:
			if l.counter > MaxLogsPerSec {
				l.heavyLoad = true
			} else {
				l.heavyLoad = false
			}
			l.counter = 0
		case <- stopC:
			ticker.Stop()
			exitLoop = true
		}
	}

	stopMsg.Report()
}

func (l *memLogger) Close() {
	l.stopBc.SendAndWait(struct{}{})
}
