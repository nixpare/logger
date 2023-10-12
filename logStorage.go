package logger

import (
	"bufio"
	"encoding/json"
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
	getSpecificLogs(logs []int) []Log
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

	fls.f, err = createNewFileStorage(fls, 0)
	if err != nil {
		return nil, err
	}

	return fls, nil
}

func createNewFileStorage(fls *fileLogStorage, index int) (*os.File, error) {
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

			f, err := createNewFileStorage(fls, fls.chunks)
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

func openNewFileStorage(fls *fileLogStorage, index int) (*os.File, error) {
	Debug("Open")
	return os.Open(fls.dir + "/" + fls.prefix + fmt.Sprintf("%04d", index))
}

func (fls *fileLogStorage) getLog(index int) Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()

	if index >= fls.n - LogChunkSize {
		index = index - (fls.n - LogChunkSize) + fls.cacheHead
		index %= LogChunkSize
		return fls.cache[index]
	}

	fNum := index / LogChunkSize
	index = index % LogChunkSize

	f, err := openNewFileStorage(fls, fNum)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for i := 0; i < index; i++ {
		sc.Scan()
	}
	sc.Scan()
	
	var l Log
	err = json.Unmarshal(sc.Bytes(), &l)
	if err != nil {
		panic(err)
	}

	return l
}

type interval struct {
	start, end int
}

func (fls fileLogStorage) splitRequestRange(start, end int) (res []interval) {
	if end-1 >= fls.n - LogChunkSize {
		if start < fls.n - LogChunkSize {
			defer func(end int) {
				res = append(res, interval{
					start: fls.n - LogChunkSize,
					end: end,
				})
			}(end)
			
			end = fls.n - LogChunkSize
		} else {
			res = append(res, interval{
				start: start,
				end: end,
			})
			return res
		}
	}

	inter := interval{ start: start, end: start+1 }
	
	for i := start+1; i < end; i++ {
		if i % LogChunkSize == 0 {
			res = append(res, inter)
			inter = interval{ start: i, end: i+1 }
		} else {
			inter.end ++
		}
	}
	res = append(res, inter)

	return res
}

func (fls*fileLogStorage) getLogs(start, end int) []Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()

	inter := fls.splitRequestRange(start, end)
	res := make([]Log, 0, end-start)

	for _, x := range inter {
		if x.start >= fls.n - LogChunkSize {
			for i := x.start; i < x.end; i++ {
				res = append(res, fls.getLog(i))
			}
		} else {
			fNum := x.start / LogChunkSize

			f, err := openNewFileStorage(fls, fNum)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			sc := bufio.NewScanner(f)
			for i := fNum * LogChunkSize; i < x.start; i++ {
				sc.Scan()
			}

			for i := x.start; i < x.end; i++ {
				sc.Scan()
				
				var l Log
				err = json.Unmarshal(sc.Bytes(), &l)
				if err != nil {
					panic(err)
				}

				res = append(res, l)
			}
		}
	}

	return res
}

func splitRequestSingle(logs []int) [][]int {
	res := make([][]int, 0)

	inter := []int{logs[0]}
	for i := 1; i < len(logs); i++ {
		if logs[i] / LogChunkSize == inter[0] / LogChunkSize {
			inter = append(inter, logs[i])
			continue
		}

		res = append(res, inter)
		inter = []int{logs[i]}
	}
	res = append(res, inter)

	return res
}

func (fls*fileLogStorage) getSpecificLogs(logs []int) []Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()

	inter := splitRequestSingle(logs)
	res := make([]Log, 0, len(logs))

	Debug(inter)

	for _, i := range inter {
		if i[0] / LogChunkSize == fls.chunks {
			for _, p := range i {
				res = append(res, fls.getLog(p))
			}
		} else {
			fNum := i[0] / LogChunkSize

			f, err := openNewFileStorage(fls, fNum)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			sc := bufio.NewScanner(f)
			lastRead := (fNum * LogChunkSize) - 1

			for _, p := range i {
				for j := lastRead + 1; j < p; j++ {
					sc.Scan()
				}

				sc.Scan()
				lastRead = p

				var l Log
				err = json.Unmarshal(sc.Bytes(), &l)
				if err != nil {
					panic(err)
				}

				res = append(res, l)
			}
		}
	}

	return res
}

func (fls *fileLogStorage) nLogs() int {
	return fls.n
}
