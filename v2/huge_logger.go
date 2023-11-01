package logger

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/nixpare/comms"
)

var MaxMemUsage uint64 = 2 * 1000 * 1000 * 1000

type hugeLogger struct {
	out            io.Writer
	hls            *hugeLogStorage
	tags           []string
	extrasDisabled bool
	counter        int
	heavyLoad      bool
	lastWrote      int
	rwm            *sync.RWMutex
	alignM         *sync.Mutex
	stopBc         *comms.Broadcaster[struct{}]
}

func (l *hugeLogger) newLog(log Log, writeOutput bool) int {
	l.counter++
	log.addTags(l.tags...)

	l.rwm.Lock()

	l.hls.addLog(log)
	p := l.hls.n - 1

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

func (l *hugeLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
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
	return l.hls.n
}

func (l *hugeLogger) Out() io.Writer {
	return l.out
}

func (l *hugeLogger) GetLog(index int) Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.hls.getLog(index)
}

func (l *hugeLogger) GetLastNLogs(n int) []Log {
	tot := l.NLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *hugeLogger) GetLogs(start, end int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.hls.getLogs(start, end)
}

func (l *hugeLogger) GetSpecificLogs(logs []int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.hls.getSpecificLogs(logs)
}

func (l *hugeLogger) AsStdout() io.Writer {
	return asStdout(l)
}

func (l *hugeLogger) AsStderr() io.Writer {
	return asStderr(l)
}

func (l *hugeLogger) FixedLogger(level LogLevel) io.Writer {
	return fixedLogger(l, level)
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
	return newCloneLogger(l, out, parentOut, tags, l.extrasDisabled)
}

func memUsageExceeded() bool {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem.Alloc > MaxMemUsage
}

func (l *hugeLogger) checkHeavyLoad() {
	ticker := time.NewTicker(ScanInterval)
	var exitLoop bool

	stopC := make(chan struct{})
	defer close(stopC)

	var stopMsg comms.BroadcastMessage[struct{}]
	go func() {
		stopMsg = l.stopBc.Listen()
		stopC <- struct{}{}
	}()

	var alignInProgress, memRecoveryInProgress bool
	var releaseCounter int

	for !exitLoop {
		select {
		case <-ticker.C:
			if memUsageExceeded() && len(l.hls.buffer) != 0 {
				if !memRecoveryInProgress {
					memRecoveryInProgress = true
					go func() {
						l.hls.alignStorage(true)
						memRecoveryInProgress = false
					}()
				}
			}

			if l.counter > MaxLogsPerScan {
				releaseCounter = 0
				l.heavyLoad = true
				l.hls.heavyLoad = true
			} else {
				releaseCounter ++

				if releaseCounter > NegativeScansBeforeAlign {
					l.heavyLoad = false
					l.hls.heavyLoad = false
	
					if !alignInProgress {
						alignInProgress = true
						go func() {
							l.alignOutput(false)
							l.hls.alignStorage(false)
							alignInProgress = false
						}()
					}
				}
			}

			l.counter = 0
		case <-stopC:
			ticker.Stop()
			exitLoop = true

			l.alignOutput(true)
			l.hls.alignStorage(true)
		}
	}

	stopMsg.Report()
}

func (l *hugeLogger) EnableHeavyLoadDetection() {
	if l.out != nil {
		go l.checkHeavyLoad()
	}
}

func (l *hugeLogger) Close() {
	l.stopBc.SendAndWait(struct{}{})
}

func (l *hugeLogger) alignOutput(empty bool) {
	l.alignM.Lock()
	defer l.alignM.Unlock()

	if l.NLogs() == 0 {
		return
	}

	logs := l.GetLastNLogs(l.NLogs() - l.lastWrote - 1)

	for {
		if !empty && l.heavyLoad {
			break
		}

		if len(logs) == 0 {
			break
		}

		v := logs
		if len(v) > MaxLogsPerScan {
			v = v[:MaxLogsPerScan]
		}
		logs = logs[len(v):]

		for _, log := range v {
			logToOut(l, log, l.extrasDisabled)
		}

		l.rwm.Lock()
		l.lastWrote += len(v)
		l.rwm.Unlock()
	}
}
