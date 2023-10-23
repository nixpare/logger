package logger

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nixpare/comms"
)

type hugeLogger struct {
	out             io.Writer
	fls             *hugeLogStorage
	tags            []string
	extrasDisabled  bool
	counter         int
	heavyLoad       bool
	lastWrote       int
	writingM        *sync.Mutex
	stopBc          *comms.Broadcaster[struct{}]
}

func (l *hugeLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)
	p := l.fls.addLog(log)

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

func (l *hugeLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.counter++

	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *hugeLogger) Print(level LogLevel, a ...any) {
	print(l, level, a...)
}

func (l *hugeLogger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

func (l *hugeLogger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func (l *hugeLogger) NLogs() int {
	return l.fls.nLogs()
}

func (l *hugeLogger) Out() io.Writer {
	return l.out
}

func (l *hugeLogger) GetLog(index int) Log {
	return l.fls.getLog(index)
}

func (l *hugeLogger) GetLastNLogs(n int) []Log {
	tot := l.fls.nLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *hugeLogger) GetLogs(start, end int) []Log {
	return l.fls.getLogs(start, end)
}

func (l *hugeLogger) GetSpecificLogs(logs []int) []Log {
	return l.fls.getSpecificLogs(logs)
}

func (l *hugeLogger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *hugeLogger) EnableExtras() {
	l.extrasDisabled = false
}

func (l *hugeLogger) DisableExtras() {
	l.extrasDisabled = true
}

func (l *hugeLogger) Clone(out io.Writer, parentOut bool, tags ...string) Logger {
	return newCloneLogger(l, out, tags, l.extrasDisabled, parentOut)
}

func (l *hugeLogger) checkHeavyLoad() {
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

func (l *hugeLogger) EnableHeavyLoadDetection() {
	go l.checkHeavyLoad()
}

func (l *hugeLogger) Close() {
	l.stopBc.SendAndWait(struct{}{})
}

func (l *hugeLogger) alignOutput() {
	if l.out == nil || l.fls.n == 0 {
		return
	}

	l.writingM.Lock()
	defer l.writingM.Unlock()

	for {
		logs := l.fls.getLogs(l.lastWrote+1, l.fls.n)
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			logToOut(l, log, l.extrasDisabled)
		}
		l.lastWrote += len(logs)
	}
}
