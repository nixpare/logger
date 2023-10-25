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

type hugeLogStorage struct {
	n         int      // n is the number of logs stored
	chunks    int      // chunks is the number of files created to store the logs
	cache     []Log    // cache holds the most recent logs, it is a circular list
	cacheHead int      // cacheHead points to the start of the cache
	dir       string   // dir is the directory where the files are saved
	prefix    string   // prefix holds the identifier of the log files and the timestamp
	f         *os.File // f is the last log file opened for writing
	lastStored  int
	heavyLoad   bool
	buffer      map[int]*[]Log
	rwm         *sync.RWMutex
	storeM      *sync.RWMutex
}

func initHugeLogStorage(dir, prefix string) (*hugeLogStorage, error) {
	if !filepath.IsAbs(dir) {
		wd, _ := os.Getwd()
		dir = wd + "/" + dir
	}

	hls := &hugeLogStorage{
		cache:  make([]Log, 0),
		dir:    dir,
		prefix: fmt.Sprintf("%s-%s-", prefix, time.Now().Format(LogFileTimeFormat)),
		lastStored: -1,
		buffer: make(map[int]*[]Log),
		rwm:    new(sync.RWMutex),
		storeM: new(sync.RWMutex),
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("the provided path is not a directory")
	}

	hls.f, err = os.Create(hls.fileNameGeneration(0))
	if err != nil {
		return nil, err
	}

	return hls, nil
}

func (hls *hugeLogStorage) fileNameGeneration(index int) string {
	return fmt.Sprintf("%s/%s%d.%s", hls.dir, hls.prefix, index, LogFileExtension)
}

func (hls *hugeLogStorage) addLog(l Log) {
	if len(hls.cache) < LogChunkSize {
		hls.cache = append(hls.cache, l)
	} else {
		hls.cache[hls.cacheHead] = l
		hls.cacheHead = (hls.cacheHead + 1) % len(hls.cache)

		if hls.n%LogChunkSize == 0 {
			hls.f.Close()

			hls.chunks++
			f, err := os.Create(hls.fileNameGeneration(hls.chunks))
			if err != nil {
				panic(err)
			}
			hls.f = f
		}
	}

	hls.rwm.Lock()
	if !hls.heavyLoad && hls.lastStored + 1 == hls.n {
		hls.f.Write(l.JSON())
		hls.f.Write([]byte{'\n'})

		hls.lastStored ++
		hls.rwm.Unlock()
	} else {
		hls.storeM.Lock()
		hls.rwm.Unlock()

		b, ok := hls.buffer[hls.chunks]
		if !ok {
			b = newLogBuffer()
			hls.buffer[hls.chunks] = b
		}

		*b = append(*b, l)
		hls.storeM.Unlock()
	}

	hls.n ++
}

func (hls *hugeLogStorage) getLog(index int) Log {
	switch {
	case hls.n <= LogChunkSize:
		{
			return hls.cache[index]
		}
	case index >= hls.n-LogChunkSize:
		index = index - (hls.n - LogChunkSize) + hls.cacheHead
		index %= LogChunkSize
		return hls.cache[index]
	}

	fNum := index / LogChunkSize
	index = index % LogChunkSize

	f, err := os.Open(hls.fileNameGeneration(fNum))
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

func (hls hugeLogStorage) splitRequestRange(start, end int) (res []interval) {
	if end-1 >= hls.n-LogChunkSize {
		if start < hls.n-LogChunkSize {
			defer func(end int) {
				res = append(res, interval{
					start: hls.n - LogChunkSize,
					end:   end,
				})
			}(end)

			end = hls.n - LogChunkSize
		} else {
			res = append(res, interval{
				start: start,
				end:   end,
			})
			return
		}
	}

	inter := interval{start: start, end: start + 1}

	for i := start + 1; i < end; i++ {
		if i%LogChunkSize == 0 {
			res = append(res, inter)
			inter = interval{start: i, end: i + 1}
		} else {
			inter.end++
		}
	}
	res = append(res, inter)

	return
}

func (hls *hugeLogStorage) getLogs(start, end int) []Log {
	inter := hls.splitRequestRange(start, end)
	res := make([]Log, 0, end-start)

	for _, x := range inter {
		if x.start >= hls.n-LogChunkSize {
			for i := x.start; i < x.end; i++ {
				res = append(res, hls.getLog(i))
			}
		} else {
			hls.storeM.RLock()
			defer hls.storeM.RUnlock()

			fNum := x.start / LogChunkSize

			f, err := os.Open(hls.fileNameGeneration(fNum))
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
					Debug(hls.buffer)
					panic(err)
				}

				res = append(res, l)
			}
		}
	}

	return res
}

func (hls hugeLogStorage) splitRequestSingle(logs []int) (res [][]int) {
	if len(logs) == 0 {
		return
	}

	if logs[len(logs)-1] >= hls.n-LogChunkSize {
		var inter []int
		var i int

		func() {
			for i = len(logs) - 2; i >= 0 && logs[i] >= hls.n-LogChunkSize; i-- {
				defer func(p int) {
					inter = append(inter, p)
				}(logs[i])
			}
		}()
		inter = append(inter, logs[len(logs)-1])

		defer func(inter []int) {
			res = append(res, inter)
		}(inter)
		logs = logs[:i+1]
	}

	if len(logs) == 0 {
		return
	}

	inter := []int{logs[0]}
	for i := 1; i < len(logs); i++ {
		if logs[i]/LogChunkSize == inter[0]/LogChunkSize {
			inter = append(inter, logs[i])
			continue
		}

		res = append(res, inter)
		inter = []int{logs[i]}
	}
	res = append(res, inter)

	return
}

func (hls *hugeLogStorage) getSpecificLogs(logs []int) []Log {
	inter := hls.splitRequestSingle(logs)
	res := make([]Log, 0, len(logs))

	for _, i := range inter {
		if i[0] >= hls.n-LogChunkSize {
			for _, p := range i {
				res = append(res, hls.getLog(p))
			}
		} else {
			fNum := i[0] / LogChunkSize

			f, err := os.Open(hls.fileNameGeneration(fNum))
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

func (hls *hugeLogStorage) alignStorage(empty bool) {
	hls.storeM.Lock()
	defer hls.storeM.Unlock()

	if hls.n == 0 {
		return
	}

	for chunk, b := range hls.buffer {
		if !empty && hls.heavyLoad {
			break
		}

		if len(*b) == 0 {
			break
		}

		f, err := os.OpenFile(hls.fileNameGeneration(chunk), os.O_WRONLY | os.O_APPEND, 0)
		if err != nil {
			panic(err)
		}

		for _, log := range *b {
			f.Write(log.JSON())
			f.Write([]byte{'\n'})
		}

		hls.rwm.Lock()
		hls.lastStored += len(*b)
		hls.rwm.Unlock()

		logPool.Put(b)
		delete(hls.buffer, chunk)
	}
}

func Align(l Logger) {
	hl, ok := l.(*hugeLogger)
	if !ok {
		return
	}

	hl.hls.alignStorage(true)
}

func SetHeavyLoad(l Logger, value bool) {
	hl, ok := l.(*hugeLogger)
	if !ok {
		return
	}

	hl.hls.heavyLoad = value
}
