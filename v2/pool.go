package logger

import "sync"

var logPool = sync.Pool{
	New: func() any {
		v := new([]Log)
		*v = make([]Log, 0, LogChunkSize)
		return v
	},
}

func newLogBuffer() *[]Log {
	v := logPool.Get().(*[]Log)
	*v = (*v)[:0]
	return v
}