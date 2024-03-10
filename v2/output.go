package logger

import "strings"

type fixLogger struct {
	l     Logger
	level LogLevel
}

func (fl *fixLogger) Write(p []byte) (n int, err error) {
	fl.l.Print(fl.level, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
