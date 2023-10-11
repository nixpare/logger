package logger

import (
	"fmt"
	"io"
	"os"
)

type cloneLogger struct {
	parent Logger
	tags []string
	logs []*Log
	out io.Writer
	wantExtras  bool
	multiLogger bool
}

func (l *cloneLogger) newLog(log Log, writeOutput bool) *Log {
	log.addTags(l.tags...)

	/* if !writeOutput {
		l.parent.newLog(log, writeOutput)
	} else {
		if l.out == nil {
			l.parent.newLog(log, writeOutput)
		} else {
			if l.multiLogger && l.out != l.parent.Out {
				l.parent.newLog(log, writeOutput)
			} else {
				l.parent.newLog(log, false)
			}
		}
	} */
	// Equivalent to the above
	var p *Log
	if writeOutput && l.out != nil && (!l.multiLogger || l.out == l.parent.Out()) {
		p = l.parent.newLog(log, false)
	} else {
		p = l.parent.newLog(log, writeOutput)
	}

	l.logs = append(l.logs, p)

	if l.out == nil || !writeOutput {
		return p
	}

	out := l.out
	if level := log.Level(); out == os.Stdout && (level == LOG_LEVEL_WARNING || level == LOG_LEVEL_ERROR || level == LOG_LEVEL_FATAL) {
		out = os.Stderr
	}

	if ToTerminal(l.out) {
		if log.l.extra != "" && l.wantExtras {
			fmt.Fprintln(out, log.l.fullColored())
		} else {
			fmt.Fprintln(out, log.l.colored())
		}
	} else {
		if log.l.extra != "" && l.wantExtras {
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
		wantExtras: l.wantExtras,
		parent: l,
	}
}

func (l *cloneLogger) DisableExtras() {
	l.wantExtras = false
}

func (l *cloneLogger) DisableMultiLogger() {
	l.multiLogger = false
}

func (l *cloneLogger) EnableExtras() {
	l.wantExtras = true
}

func (l *cloneLogger) EnableMultiLogger() {
	l.multiLogger = true
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
		res = append(res, *l.logs[i])
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
