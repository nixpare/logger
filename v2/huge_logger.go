package logger

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/nixpare/broadcaster"
)

var MaxMemUsage uint64 = 2 * 1000 * 1000 * 1000

type HugeLogger struct {
	out            io.Writer
	hls            *hugeLogStorage
	tags           []string
	extrasDisabled bool
	counter        int
	heavyLoad      bool
	lastWrote      int
	rwm            *sync.RWMutex
	alignM         *sync.Mutex
	stopBc         *broadcaster.BroadcastWaiter[struct{}]
}

func (l *HugeLogger) newLog(log Log, writeOutput bool) int {
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

func (l *HugeLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) int {
	return l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *HugeLogger) Print(level LogLevel, a ...any) {
	print(l, level, a...)
}

func (l *HugeLogger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

func (l *HugeLogger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func (l *HugeLogger) NLogs() int {
	return l.hls.n
}

func (l *HugeLogger) Out() io.Writer {
	return l.out
}

func (l *HugeLogger) GetLog(index int) Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.hls.getLog(index)
}

func (l *HugeLogger) GetLastNLogs(n int) []Log {
	tot := l.NLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *HugeLogger) GetLogs(start, end int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.hls.getLogs(start, end)
}

func (l *HugeLogger) GetSpecificLogs(logs []int) []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	return l.hls.getSpecificLogs(logs)
}

func (l *HugeLogger) AsStdout() io.Writer {
	return asStdout(l)
}

func (l *HugeLogger) AsStderr() io.Writer {
	return asStderr(l)
}

func (l *HugeLogger) FixedLogger(level LogLevel) io.Writer {
	return fixedLogger(l, level)
}

func (l *HugeLogger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *HugeLogger) EnableExtras() {
	l.extrasDisabled = false
}

func (l *HugeLogger) DisableExtras() {
	l.extrasDisabled = true
}

func (l *HugeLogger) Clone(out io.Writer, parentOut bool, tags ...string) Logger {
	return newCloneLogger(l, out, parentOut, tags, l.extrasDisabled)
}

func memUsageExceeded() bool {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem.Alloc > MaxMemUsage
}

func (l *HugeLogger) checkHeavyLoad() {
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
				releaseCounter++

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

	stopMsg.Done()
}

func (l *HugeLogger) EnableHeavyLoadDetection() {
	if l.out != nil {
		go l.checkHeavyLoad()
	}
}

func (l *HugeLogger) Close() {
	l.stopBc.Send(struct{}{}).Wait()
}

func (l *HugeLogger) alignOutput(empty bool) {
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

func (l *HugeLogger) GetLastNLogsBuffered(n int) <-chan []Log {
	tot := l.NLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogsBuffered(tot-n, tot)
}

func (l *HugeLogger) GetLogsBuffered(start, end int) <-chan []Log {
	l.rwm.RLock()
	defer l.rwm.RUnlock()

	c := make(chan []Log)

	go func() {
		defer close(c)

		var i int
		for i = start; i+1000 < end; i += 1000 {
			c <- l.hls.getLogs(i, i+1000)
		}
		if i < end {
			c <- l.hls.getLogs(i, end)
		}
	}()

	return c
}
