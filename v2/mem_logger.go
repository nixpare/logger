package logger

import (
	"fmt"
	"io"
	"sync"
)

func (l *memLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)
	p := l.storage.addLog(log)

	if l.out == nil || !writeOutput {
		return p
	}

	if !l.heavyLoad {
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
	return l.storage.nLogs()
}

func (l *memLogger) Out() io.Writer {
	return l.out
}

func (l *memLogger) GetLog(index int) Log {
	return l.storage.getLog(index)
}

func (l *memLogger) GetLastNLogs(n int) []Log {
	tot := l.storage.nLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *memLogger) GetLogs(start, end int) []Log {
	return l.storage.getLogs(start, end)
}

func (l *memLogger) GetSpecificLogs(logs []int) []Log {
	return l.storage.getSpecificLogs(logs)
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

func (l *memLogger) Close() {
	l.stopC <- struct{}{}
}

type memLogStorage struct {
	v []Log
	rwm *sync.RWMutex
}

func (s *memLogStorage) addLog(l Log) int {
	s.rwm.Lock()
	defer s.rwm.Unlock()

	s.v = append(s.v, l)
	return len(s.v)-1
}

func (s memLogStorage) getLog(index int) Log {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.v[index]
}

func (s memLogStorage) getLogs(start, end int) []Log {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.v[start:end]
}

func (s memLogStorage) getSpecificLogs(logs []int) []Log {
	s.rwm.RLock()
	defer s.rwm.RUnlock()

	res := make([]Log, 0, len(logs))
	for _, p := range logs {
		res = append(res, s.v[p])
	}
	return res
}

func (s memLogStorage) nLogs() int {
	return len(s.v)
}
