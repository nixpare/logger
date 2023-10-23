package logger

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
}

func initFileLogStorage(dir, prefix string) (*hugeLogStorage, error) {
	if !filepath.IsAbs(dir) {
		wd, _ := os.Getwd()
		dir = wd + "/" + dir
	}

	fls := &hugeLogStorage{
		cache:  make([]Log, 0),
		dir:    dir,
		prefix: fmt.Sprintf("%s-%s-", prefix, time.Now().Format(LogFileTimeFormat)),
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

func (fls *hugeLogStorage) fileNameGeneration(index int) string {
	return fmt.Sprintf("%s/%s%d.%s", fls.dir, fls.prefix, index, LogFileExtension)
}

func (fls *hugeLogStorage) addLog(l Log) {
	if len(fls.cache) < LogChunkSize {
		fls.cache = append(fls.cache, l)
	} else {
		fls.cache[fls.cacheHead] = l
		fls.cacheHead = (fls.cacheHead + 1) % len(fls.cache)

		if fls.n%LogChunkSize == 0 {
			fls.f.Close()

			fls.chunks++
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
}

func (fls *hugeLogStorage) getLog(index int) Log {
	switch {
	case fls.n <= LogChunkSize:
		{
			return fls.cache[index]
		}
	case index >= fls.n-LogChunkSize:
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

func (fls hugeLogStorage) splitRequestRange(start, end int) (res []interval) {
	if end-1 >= fls.n-LogChunkSize {
		if start < fls.n-LogChunkSize {
			defer func(end int) {
				res = append(res, interval{
					start: fls.n - LogChunkSize,
					end:   end,
				})
			}(end)

			end = fls.n - LogChunkSize
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

func (fls *hugeLogStorage) getLogs(start, end int) []Log {
	inter := fls.splitRequestRange(start, end)
	res := make([]Log, 0, end-start)

	for _, x := range inter {
		if x.start >= fls.n-LogChunkSize {
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

func (fls hugeLogStorage) splitRequestSingle(logs []int) (res [][]int) {
	if len(logs) == 0 {
		return
	}

	if logs[len(logs)-1] >= fls.n-LogChunkSize {
		var inter []int
		var i int

		func() {
			for i = len(logs) - 2; i >= 0 && logs[i] >= fls.n-LogChunkSize; i-- {
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

func (fls *hugeLogStorage) getSpecificLogs(logs []int) []Log {
	inter := fls.splitRequestSingle(logs)
	res := make([]Log, 0, len(logs))

	for _, i := range inter {
		if i[0] >= fls.n-LogChunkSize {
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
