package logger

import (
	"fmt"
	"io"
)

// cloneLogger implements the Logger interface and basically
// maps every log is created with it (or any child in cascade)
// with the index associated with the same log for the parent,
// whichever type of Logger it is.
type cloneLogger struct {
	parent Logger
	tags []string
	logs []int
	out io.Writer
	disableExtras  bool
}

func (l *cloneLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)

	var p int
	if writeOutput && l.out != nil && l.out == l.parent.Out() {
		p = l.parent.newLog(log, false)
	} else {
		p = l.parent.newLog(log, writeOutput)
	}

	l.logs = append(l.logs, p)
	p = len(l.logs) - 1

	if l.out == nil || !writeOutput {
		return p
	}

	logToOut(l, log, l.disableExtras)

	return p
}

func (l *cloneLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *cloneLogger) Clone(out io.Writer, tags ...string) Logger {
	return &cloneLogger{
		out:  out,
		tags: tags,
		disableExtras: l.disableExtras,
		parent: l,
	}
}

func (l *cloneLogger) DisableExtras() {
	l.disableExtras = true
}

func (l *cloneLogger) EnableExtras() {
	l.disableExtras = false
}

func (l *cloneLogger) GetLog(index int) Log {
	p := l.logs[index]
	return l.parent.GetLog(p)
}

func (l *cloneLogger) GetLastNLogs(n int) []Log {
	tot := len(l.logs)
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *cloneLogger) GetLogs(start int, end int) []Log {
	logsToParent := make([]int, 0, end-start)
	logsToParent = append(logsToParent, l.logs[start:end]...)
	return l.parent.GetSpecificLogs(logsToParent)
}

func (l *cloneLogger) GetSpecificLogs(logs []int) []Log {
	logsToParent := make([]int, 0, len(logs))
	for _, p := range logs {
		logsToParent = append(logsToParent, l.logs[p])
	}
	return l.parent.GetSpecificLogs(logsToParent)
}

func (l *cloneLogger) NLogs() int {
	return len(l.logs)
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
