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
	out            io.Writer
	v              []int
	tags           []string
	extrasDisabled bool
	parent         Logger
	parentOut      bool
	counter        int
	heavyLoad      bool
	lastWrote      int
	rwm            *sync.RWMutex // rwm handles access to v and lastWrote
	outM           *sync.Mutex   // outM handles access to out
	stopBc         *comms.Broadcaster[struct{}]
}

func newCloneLogger(parent Logger, out io.Writer, parentOut bool, tags []string, extrasDisabled bool) *cloneLogger {
	l := &cloneLogger{
		out:            out,
		v:              make([]int, 0),
		tags:           tags,
		extrasDisabled: extrasDisabled,
		parent:         parent,
		parentOut:      parentOut,
		lastWrote:      -1,
		rwm:            new(sync.RWMutex),
		outM:           new(sync.Mutex),
		stopBc:         comms.NewBroadcaster[struct{}](),
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
	defer l.rwm.Unlock()

	l.v = append(l.v, p)
	p = len(l.v) - 1

	if !l.heavyLoad && l.lastWrote == p-1 {
		l.lastWrote = p
		if l.out != nil && writeOutput {
			l.outM.Lock()
			logToOut(l, log, l.extrasDisabled)
			l.outM.Unlock()
		}
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
	return newCloneLogger(l, out, parentOut, tags, l.extrasDisabled)
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

	for !exitLoop {
		select {
		case <-ticker.C:
			if l.counter > MaxLogsPerSec {
				l.heavyLoad = true
			} else {
				l.heavyLoad = false
				l.alignOutput()
			}
			l.counter = 0
		case <-stopC:
			exitLoop = true
			ticker.Stop()
			l.alignOutput()
		}
	}

	stopMsg.Report()
}

func (l *cloneLogger) EnableHeavyLoadDetection() {
	if l.out != nil {
		go l.checkHeavyLoad()
	}
}

func (l *cloneLogger) Close() {
	l.stopBc.SendAndWait(struct{}{})
}

func (l *cloneLogger) alignOutput() {
	l.outM.Lock()
	defer l.outM.Unlock()

	if len(l.v) == 0 {
		return
	}

	for {
		logs := l.GetLastNLogs(l.NLogs() - l.lastWrote - 1)

		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			logToOut(l, log, l.extrasDisabled)
		}

		l.rwm.Lock()
		l.lastWrote += len(logs)
		l.rwm.Unlock()
	}
}
