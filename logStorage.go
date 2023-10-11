package logger

import "sync"

type logStorage interface {
	addLog(l Log) *Log
	getLogs(start, end int) []Log
	nLogs() int
}

type memLogStorage struct {
	a []Log
	rwm *sync.RWMutex
}

func (s *memLogStorage) addLog(l Log) *Log {
	s.rwm.Lock()
	defer s.rwm.Unlock()

	s.a = append(s.a, l)
	return &s.a[len(s.a)-1]
}

func (s memLogStorage) getLogs(start, end int) []Log {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.a[start:end]
}

func (s memLogStorage) nLogs() int {
	return len(s.a)
}

/* type fileLogStorage struct {

} */
