package logger

type outLogger struct {
	l Logger
}

func (ol *outLogger) Write(p []byte) (n int, err error) {
	ol.l.AddLog(log_level_stdout, string(p), "", true)
	return len(p), nil
}

type errLogger struct {
	l Logger
}

func (ol *errLogger) Write(p []byte) (n int, err error) {
	ol.l.AddLog(log_level_stderr, string(p), "", true)
	return len(p), nil
}

type fixLogger struct {
	l     Logger
	level LogLevel
}

func (fl *fixLogger) Write(p []byte) (n int, err error) {
	fl.l.Print(fl.level, string(p))
	return len(p), nil
}
