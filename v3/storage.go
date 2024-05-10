package logger

type storage interface {
	logs() int
	addLog(l Log)
	getLog(index int) Log
	getLogs(start, end int) []Log
	getSpecificLogs(logs []int) []Log
	align(empty bool)
	needsFlushing() bool
	enableEavyLoad()
	disableEavyLoad()
}

type memLogStorage struct {
	v []Log
}

func (mls *memLogStorage) logs() int {
	return len(mls.v)
}

func (mls *memLogStorage) addLog(l Log) {
	mls.v = append(mls.v, l)
}

func (mls *memLogStorage) getLog(index int) Log {
	return mls.v[index]
}

func (mls *memLogStorage) getLogs(start, end int) []Log {
	return mls.v[start:end]
}

func (mls *memLogStorage) getSpecificLogs(logs []int) []Log {
	res := make([]Log, 0, len(logs))
	for _, i := range logs {
		res = append(res, mls.v[i])
	}
	return res
}

func (mls *memLogStorage) align(empty bool) {}

func (mls *memLogStorage) needsFlushing() bool {
	return false
}

func (mls *memLogStorage) enableEavyLoad() {}

func (mls *memLogStorage) disableEavyLoad() {}

type cloneLogStorage struct {
	parent *Logger
	parentOut bool
	v []int
}

func (cls *cloneLogStorage) logs() int {
	return len(cls.v)
}

func (cls *cloneLogStorage) addLog(log Log) {
	var p int

	if !cls.parentOut {
		p = cls.parent.newLog(log, false)
	} else {
		p = cls.parent.newLog(log, !log.avoidLogging)
	}

	cls.v = append(cls.v, p)
}

func (cls *cloneLogStorage) getLog(index int) Log {
	return cls.parent.GetLog(cls.v[index])
}

func (cls *cloneLogStorage) getLogs(start, end int) []Log {
	logsToParent := make([]int, 0, end-start)
	logsToParent = append(logsToParent, cls.v[start:end]...)

	return cls.parent.GetSpecificLogs(logsToParent)
}

func (cls *cloneLogStorage) getSpecificLogs(logs []int) []Log {
	logsToParent := make([]int, 0, len(logs))
	for _, p := range logs {
		logsToParent = append(logsToParent, cls.v[p])
	}

	return cls.parent.GetSpecificLogs(logsToParent)
}

func (cls *cloneLogStorage) align(empty bool) {}

func (cls *cloneLogStorage) needsFlushing() bool {
	return false
}

func (cls *cloneLogStorage) enableEavyLoad() {}

func (cls *cloneLogStorage) disableEavyLoad() {}
