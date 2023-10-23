package logger

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nixpare/comms"
)

// cloneLogger implements the Logger interface and basically
// maps every log is created with it (or any child in cascade)
// with the index associated with the same log for the parent,
// whichever type of Logger it is.
type cloneLogger struct {
	parent          Logger
	tags            []string
	v               []int
	rwm             *sync.RWMutex
	out             io.Writer
	extrasDisabled  bool
	parentOut       bool
	counter         int
	heavyLoad       bool
	lastWrote       int
	writingM        *sync.Mutex
	stopBc          *comms.Broadcaster[struct{}]
}

func newCloneLogger(parent Logger, out io.Writer, tags []string, extrasDisabled bool, parentOut bool) *cloneLogger {
	l := &cloneLogger{
		parent:         parent,
		out:            out,
		v:              make([]int, 0),
		rwm:            new(sync.RWMutex),
		tags:           tags,
		extrasDisabled: extrasDisabled,
		parentOut:      parentOut,
		writingM:       new(sync.Mutex),
		stopBc:         comms.NewBroadcaster[struct{}](),
	}

	if out != nil {
		go l.checkHeavyLoad()
	}
	
	return l
}

func (l *cloneLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)

	var p int
	if !l.parentOut {
		p = l.parent.newLog(log, false)
	} else {
		p = l.parent.newLog(log, writeOutput)
	}

	l.rwm.Lock()
	l.v = append(l.v, p)
	p = len(l.v) - 1
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

func (l *cloneLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.counter++

	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *cloneLogger) Clone(out io.Writer, parentOut bool, tags ...string) Logger {
	return newCloneLogger(l, out, tags, l.extrasDisabled, parentOut)
}

func (l *cloneLogger) DisableExtras() {
	l.extrasDisabled = true
}

func (l *cloneLogger) EnableExtras() {
	l.extrasDisabled = false
}

func (l *cloneLogger) GetLog(index int) Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	p := l.v[index]
	return l.parent.GetLog(p)
}

func (l *cloneLogger) GetLastNLogs(n int) []Log {
	tot := len(l.v)
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *cloneLogger) GetLogs(start int, end int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	logsToParent := make([]int, 0, end-start)
	logsToParent = append(logsToParent, l.v[start:end]...)
	return l.parent.GetSpecificLogs(logsToParent)
}

func (l *cloneLogger) GetSpecificLogs(logs []int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()
	
	logsToParent := make([]int, 0, len(logs))
	for _, p := range logs {
		logsToParent = append(logsToParent, l.v[p])
	}
	return l.parent.GetSpecificLogs(logsToParent)
}

func (l *cloneLogger) NLogs() int {
	return len(l.v)
}

func (l *cloneLogger) Out() io.Writer {
	return l.out
}

func (l *cloneLogger) Print(level LogLevel, a ...any) {
	print(l, level, a...)
}

func (l *cloneLogger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

func (l *cloneLogger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func (l *cloneLogger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *cloneLogger) checkHeavyLoad() {
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

func (l *cloneLogger) Close() {
	l.stopBc.SendAndWait(struct{}{})
}

func (l *cloneLogger) alignOutput() {
	if len(l.v) == 0 {
		return
	}

	l.writingM.Lock()
	defer l.writingM.Unlock()

	for {
		logs := l.GetLastNLogs(l.NLogs() - l.lastWrote - 1)
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			logToOut(l, log, l.extrasDisabled)
		}
		l.lastWrote += len(logs)
	}
}
