package logger

import (
	"fmt"
	"io"
	"os"
)

type clonedLog struct {
	parent Logger
	index  int
}

type cloneLogger struct {
	parent Logger
	tags []string
	logs []clonedLog
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

	l.logs = append(l.logs, clonedLog{
		parent: l.parent,
		index: p,
	})
	p = len(l.logs) - 1

	if l.out == nil || !writeOutput {
		return p
	}

	out := l.out
	if level := log.Level(); out == os.Stdout && (level == LOG_LEVEL_WARNING || level == LOG_LEVEL_ERROR || level == LOG_LEVEL_FATAL) {
		out = os.Stderr
	}

	if ToTerminal(l.out) {
		if log.l.extra != "" && !l.disableExtras {
			fmt.Fprintln(out, log.l.fullColored())
		} else {
			fmt.Fprintln(out, log.l.colored())
		}
	} else {
		if log.l.extra != "" && !l.disableExtras {
			fmt.Fprintln(out, log.l.full())
		} else {
			fmt.Fprintln(out, log.l.String())
		}
	}

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
	cl := l.logs[index]
	return cl.parent.GetLog(cl.index)
}

func (l *cloneLogger) GetLastNLogs(n int) []Log {
	tot := len(l.logs)
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *cloneLogger) GetLogs(start int, end int) []Log {
	res := make([]Log, 0, end-start)
	for i := start; i < end; i++ {
		cl := l.logs[i]
		res = append(res, cl.parent.GetLog(cl.index))
	}
	return res
}

func (l *cloneLogger) NLogs() int {
	return len(l.logs)
}

func (l *cloneLogger) Out() io.Writer {
	return l.out
}

func (l *cloneLogger) Print(level LogLevel, a ...any) {
	print(l, level, a)
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
