package logger

import "sync"

var logPoolChunkSize = sync.Pool{
	New: func() any {
		v := new([]Log)
		*v = make([]Log, 0, LogChunkSize)
		return v
	},
}

func newChunkSizeBuffer() *[]Log {
	v := logPoolChunkSize.Get().(*[]Log)
	*v = (*v)[:0]
	return v
}
