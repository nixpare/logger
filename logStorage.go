package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var LogFileTimeFormat = "06.01.02-15.04.05"

var LogChunkSize = 4

type logStorage interface {
	addLog(l Log) int
	getLog(index int) Log
	getLogs(start, end int) []Log
	nLogs() int
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

func (s memLogStorage) nLogs() int {
	return len(s.v)
}

type fileLogStorage struct {
	n int
	chunks int
	cache []Log
	cacheHead int
	dir string
	prefix string
	f *os.File
	rwm *sync.RWMutex
}

func initFileLogStorage(dir, prefix string) (*fileLogStorage, error) {
	if !filepath.IsAbs(dir) {
		wd, _ := os.Getwd()
		dir = wd + "/" + dir
	}
	
	fls := &fileLogStorage{
		cache: make([]Log, 0),
		dir: dir,
		prefix: fmt.Sprintf("%s-%s-", prefix, time.Now().Format(LogFileTimeFormat)),
		rwm: new(sync.RWMutex),
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("the provided path is not a directory")
	}

	fls.f, err = openNewFileStorage(fls, 0)
	if err != nil {
		return nil, err
	}

	return fls, nil
}

func openNewFileStorage(fls *fileLogStorage, index int) (*os.File, error) {
	return os.Create(fls.dir + "/" + fls.prefix + fmt.Sprintf("%04d", index))
}

func (fls *fileLogStorage) addLog(l Log) int {
	fls.rwm.Lock()
	defer fls.rwm.Unlock()

	p := fls.n
	if len(fls.cache) < LogChunkSize {
		fls.cache = append(fls.cache, l)
	} else {
		fls.cache[fls.cacheHead] = l
		fls.cacheHead = (fls.cacheHead + 1) % len(fls.cache)

		if fls.n % LogChunkSize == 0 {
			fls.f.Close()
			fls.chunks ++

			f, err := openNewFileStorage(fls, fls.chunks)
			if err != nil {
				panic(err)
			}
			fls.f = f
		}
	}
	fls.n ++

	fls.f.Write(l.JSON())
	fls.f.Write([]byte{'\n'})
	return p
}

func (fls fileLogStorage) getLog(index int) Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()

	if index > fls.n - LogChunkSize {
		index = index - (fls.n - LogChunkSize) + fls.cacheHead
		index %= LogChunkSize
		return fls.cache[index]
	}

	return fls.cache[index]
}

func (fls fileLogStorage) getLogs(start, end int) []Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()
	return fls.cache[start:end]
}

func (fls fileLogStorage) nLogs() int {
	return fls.n
}
