package logger

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nixpare/comms"
)

type memLogger struct {
	out             io.Writer
	v               []Log
	rwm             *sync.RWMutex
	tags            []string
	extrasDisabled  bool
	counter         int
	heavyLoad       bool
	lastWrote       int
	writingM        *sync.Mutex
	stopBc          *comms.Broadcaster[struct{}]
}

func (l *memLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)

	l.rwm.Lock()
	l.v = append(l.v, log)
	p := len(l.v) - 1
	l.rwm.Unlock()

	if l.out == nil || !writeOutput {
		return p
	}

	l.writingM.Lock()
	defer l.writingM.Unlock()

	if !l.heavyLoad && l.lastWrote == p-1 {
		l.lastWrote = p
		logToOut(l, log, l.extrasDisabled)
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
	l.extrasDisabled = false
}

func (l *memLogger) DisableExtras() {
	l.extrasDisabled = true
}

func (l *memLogger) Clone(out io.Writer, parentOut bool, tags ...string) Logger {
	return newCloneLogger(l, out, tags, l.extrasDisabled, parentOut)
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
	
	var alignInProgress bool

	for !exitLoop {
		select {
		case <- ticker.C:
			if l.counter > MaxLogsPerSec {
				l.heavyLoad = true
			} else {
				if !alignInProgress {
					go func() {
						alignInProgress = true
						l.alignOutput()
						alignInProgress = false
					}()
				}
				
				l.heavyLoad = false
			}
			l.counter = 0
		case <- stopC:
			ticker.Stop()
			l.alignOutput()
			exitLoop = true
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

func (l *memLogger) alignOutput() {
	if len(l.v) == 0 {
		return
	}

	l.writingM.Lock()
	defer l.writingM.Unlock()

	for {
		logs := l.v[l.lastWrote+1:]
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			logToOut(l, log, l.extrasDisabled)
		}
		l.lastWrote += len(logs)
	}
}
