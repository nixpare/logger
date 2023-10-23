package logger

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type hugeLogger struct {
	out            io.Writer
	storage        *fileLogStorage
	tags           []string
	disableExtras  bool
}

// NewLogger creates a logger that keeps in memory the most recent logs and
// saves everything in files divided in clusters. The dir parameter tells the
// logger in which directory to save the logs' files. The prefix, instead, tells
// the logger how to name the files. Read the Logger interface docs for other informations
func NewHugeLogger(out io.Writer, dir string, prefix string, tags ...string) (Logger, error) {
	fls, err := initFileLogStorage(dir, prefix)
	if err != nil {
		return nil, err
	}

	return &hugeLogger{
		out:     out,
		storage: fls,
		tags:    tags,
	}, nil
}

func (l *hugeLogger) newLog(log Log, writeOutput bool) int {
	log.addTags(l.tags...)
	p := l.storage.addLog(log)

	if l.out == nil || !writeOutput {
		return p
	}

	logToOut(l, log, l.disableExtras)
	return p
}

func (l *hugeLogger) AddLog(level LogLevel, message string, extra string, writeOutput bool) {
	l.newLog(Log{
		l: newLog(level, message, extra),
	}, writeOutput)
}

func (l *hugeLogger) Print(level LogLevel, a ...any) {
	print(l, level, a...)
}

func (l *hugeLogger) Printf(level LogLevel, format string, a ...any) {
	l.Print(level, fmt.Sprintf(format, a...))
}

func (l *hugeLogger) Debug(a ...any) {
	l.Print(LOG_LEVEL_DEBUG, a...)
}

func (l *hugeLogger) NLogs() int {
	return l.storage.nLogs()
}

func (l *hugeLogger) Out() io.Writer {
	return l.out
}

func (l *hugeLogger) GetLog(index int) Log {
	return l.storage.getLog(index)
}

func (l *hugeLogger) GetLastNLogs(n int) []Log {
	tot := l.storage.nLogs()
	if n > tot {
		n = tot
	}
	return l.GetLogs(tot-n, tot)
}

func (l *hugeLogger) GetLogs(start, end int) []Log {
	return l.storage.getLogs(start, end)
}

func (l *hugeLogger) GetSpecificLogs(logs []int) []Log {
	return l.storage.getSpecificLogs(logs)
}

func (l *hugeLogger) Write(p []byte) (n int, err error) {
	return write(l, p)
}

func (l *hugeLogger) EnableExtras() {
	l.disableExtras = false
}

func (l *hugeLogger) DisableExtras() {
	l.disableExtras = true
}

func (l *hugeLogger) Clone(out io.Writer, tags ...string) Logger {
	return &cloneLogger{
		out:           out,
		tags:          tags,
		disableExtras: l.disableExtras,
		parent:        l,
	}
}

func (l *hugeLogger) Close() {
	panic("To be implemented")
}

type fileLogStorage struct {
	n int 				// n is the number of logs stored
	chunks int 			// chunks is the number of files created to store the logs
	cache []Log 		// cache holds the most recent logs, it is a circular list
	cacheHead int 		// cacheHead points to the start of the cache
	dir string 			// dir is the directory where the files are saved
	prefix string 		// prefix holds the identifier of the log files and the timestamp
	f *os.File 			// f is the last log file opened for writing
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

	fls.f, err = os.Create(fls.fileNameGeneration(0))
	if err != nil {
		return nil, err
	}

	return fls, nil
}

func (fls *fileLogStorage) fileNameGeneration(index int) string {
	return fmt.Sprintf("%s/%s%d.%s", fls.dir, fls.prefix, index, LogFileExtension)
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
			f, err := os.Create(fls.fileNameGeneration(fls.chunks))
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

func (fls *fileLogStorage) getLog(index int) Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()

	switch {
	case fls.n <= LogChunkSize: {
		return fls.cache[index]
	}
	case index >= fls.n - LogChunkSize:
		index = index - (fls.n - LogChunkSize) + fls.cacheHead
		index %= LogChunkSize
		return fls.cache[index]
	}

	fNum := index / LogChunkSize
	index = index % LogChunkSize

	f, err := os.Open(fls.fileNameGeneration(fNum))
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
			return
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

	return
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

			f, err := os.Open(fls.fileNameGeneration(fNum))
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

func (fls fileLogStorage) splitRequestSingle(logs []int) (res [][]int) {
	if len(logs) == 0 {
		return
	}

	if logs[len(logs)-1] >= fls.n - LogChunkSize {
		var inter []int
		var i int

		func() {
			for i = len(logs)-2; i >= 0 && logs[i] >= fls.n - LogChunkSize; i-- {
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
		if logs[i] / LogChunkSize == inter[0] / LogChunkSize {
			inter = append(inter, logs[i])
			continue
		}

		res = append(res, inter)
		inter = []int{logs[i]}
	}
	res = append(res, inter)

	return
}

func (fls*fileLogStorage) getSpecificLogs(logs []int) []Log {
	fls.rwm.RLock()
	defer fls.rwm.RUnlock()

	inter := fls.splitRequestSingle(logs)
	res := make([]Log, 0, len(logs))

	for _, i := range inter {
		if i[0] >= fls.n - LogChunkSize {
			for _, p := range i {
				res = append(res, fls.getLog(p))
			}
		} else {
			fNum := i[0] / LogChunkSize

			f, err := os.Open(fls.fileNameGeneration(fNum))
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

