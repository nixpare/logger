package logger

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var (
	TimeFormat = "2006-01-02 15:04:05.00" // TimeFormat defines which timestamp to use with the logs. It can be modified.
)

// LogLevel defines the severity of a Log. See the constants
type LogLevel int

const (
	LOG_LEVEL_BLANK LogLevel = iota
	LOG_LEVEL_INFO
	LOG_LEVEL_DEBUG
	LOG_LEVEL_WARNING
	LOG_LEVEL_ERROR
	LOG_LEVEL_FATAL
)

func (level LogLevel) String() string {
	switch level {
	case LOG_LEVEL_BLANK:
		return ""
	case LOG_LEVEL_INFO:
		return "   Info"
	case LOG_LEVEL_DEBUG:
		return "  Debug"
	case LOG_LEVEL_WARNING:
		return "Warning"
	case LOG_LEVEL_ERROR:
		return "  Error"
	case LOG_LEVEL_FATAL:
		return "  Fatal"
	default:
		return "  ???  "
	}
}

func (level LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.TrimSpace(strings.ToLower(level.String())))
}

func (level *LogLevel) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch s {
	case "":
		*level = LOG_LEVEL_BLANK
	case "info":
		*level = LOG_LEVEL_INFO
	case "debug":
		*level = LOG_LEVEL_DEBUG
	case "warning":
		*level = LOG_LEVEL_WARNING
	case "error":
		*level = LOG_LEVEL_ERROR
	case "fatal":
		*level = LOG_LEVEL_FATAL
	default:
		*level = -1
	}

	return nil
}

type log struct {
	id      string
	level   LogLevel  // Level is the Log severity (INFO - DEBUG - WARNING - ERROR - FATAL)
	date    time.Time // Date is the timestamp of the log creation
	message string    // Message is the main message that should summarize the event
	extra   string    // Extra should hold any extra information provided for deeper understanding of the event
}

func (l log) cleanMessage() string {
	return strings.TrimSpace(RemoveTerminalColors(l.message))
}

func (l log) cleanExtra() string {
	return strings.TrimSpace(RemoveTerminalColors(l.extra))
}

func newLog(level LogLevel, message string, extra string) *log {
	t := time.Now()

	return &log{
		id: fmt.Sprintf(
			"%02d%02d%02d%02d%02d%02d%03d",
			t.Year()%100, t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), rand.Intn(1000),
		),
		level: level, date: t,
		message: message, extra: extra,
	}
}

func (l log) String() string {
	if l.level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"[%v] - %s",
			l.date.Format(TimeFormat),
			l.cleanMessage(),
		)
	}

	return fmt.Sprintf(
		"[%v] - %v: %s",
		l.date.Format(TimeFormat),
		l.level, l.cleanMessage(),
	)
}

func (l log) colored() string {
	var color string
	switch l.level {
	case LOG_LEVEL_INFO:
		color = BRIGHT_CYAN_COLOR
	case LOG_LEVEL_DEBUG:
		color = DARK_MAGENTA_COLOR
	case LOG_LEVEL_WARNING:
		color = DARK_YELLOW_COLOR
	case LOG_LEVEL_ERROR:
		color = DARK_RED_COLOR
	case LOG_LEVEL_FATAL:
		color = BRIGHT_RED_COLOR
	}

	if l.level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"%s[%v]%s - %s%s",
			BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
			l.message, DEFAULT_COLOR,
		)
	}

	return fmt.Sprintf(
		"%s[%v]%s - %s%v%s: %s%s",
		BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
		color, l.level, DEFAULT_COLOR,
		l.message, DEFAULT_COLOR,
	)
}

// full is like String(), but appends all the extra information
// associated with the log instance
func (l log) full() string {
	if l.extra == "" {
		return l.String()
	}

	if l.level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"[%v] - %s\n%s",
			l.date.Format(TimeFormat),
			l.cleanMessage(), IndentString(l.cleanExtra(), 4),
		)
	}

	return fmt.Sprintf(
		"[%v] - %v: %s\n%s",
		l.date.Format(TimeFormat), l.level,
		l.cleanMessage(), IndentString(l.cleanExtra(), 4),
	)
}

// Full is like String(), but appends all the extra information
// associated with the log instance
func (l log) fullColored() string {
	if l.extra == "" {
		return l.colored()
	}

	var color string
	switch l.level {
	case LOG_LEVEL_INFO:
		color = BRIGHT_CYAN_COLOR
	case LOG_LEVEL_DEBUG:
		color = DARK_MAGENTA_COLOR
	case LOG_LEVEL_WARNING:
		color = DARK_YELLOW_COLOR
	case LOG_LEVEL_ERROR:
		color = DARK_RED_COLOR
	case LOG_LEVEL_FATAL:
		color = BRIGHT_RED_COLOR
	}

	if l.level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"%s[%v]%s - %s\n%s%s",
			BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
			l.message, IndentString(l.extra, 4), DEFAULT_COLOR,
		)
	}

	return fmt.Sprintf(
		"%s[%v]%s - %s%v%s: %s\n%s%s",
		BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
		color, l.level, DEFAULT_COLOR,
		l.message, IndentString(l.extra, 4), DEFAULT_COLOR,
	)
}

// Log is the structure that can be will store any log reported
// with Logger. It keeps the error severity level (see the constants)
// the date it was created and the message associated with it (probably
// an error). It also has the optional field "extra" that can be used to
// store additional information
type Log struct {
	l    *log
	tags []string
}

func (l Log) ID() string {
	return l.l.id
}

func (l Log) Level() LogLevel {
	return l.l.level
}

func (l Log) Date() time.Time {
	return l.l.date
}

func (l Log) Message() string {
	return l.l.cleanMessage()
}

func (l Log) RawMessage() string {
	return l.l.message
}

func (l Log) Extra() string {
	return l.l.cleanExtra()
}

func (l Log) RawExtra() string {
	return l.l.extra
}

func (l Log) Tags() []string {
	return l.tags
}

func (l *Log) addTags(tags ...string) {
loop:
	for _, tag := range tags {
		tag = strings.ToLower(tag)

		for _, lTags := range l.tags {
			if tag == lTags {
				continue loop
			}
		}

		l.tags = append(l.tags, tag)
	}
}

func (l Log) Match(tags ...string) bool {
	for _, matchTag := range tags {
		var hasMatch bool
		for _, logTag := range l.tags {
			if strings.ToLower(matchTag) == logTag {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			return false
		}
	}
	return true
}

func (l Log) MatchAny(tags ...string) bool {
	for _, matchTag := range tags {
		for _, logTag := range l.tags {
			if strings.ToLower(matchTag) == logTag {
				return true
			}
		}
	}
	return false
}

func (l Log) LevelMatchAny(levels ...LogLevel) bool {
	for _, level := range levels {
		if l.Level() == level {
			return true
		}
	}
	return false
}

type logJSON struct {
	ID      string    `json:"id"`
	Level   LogLevel  `json:"level"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
	Extra   string    `json:"extra"`
	Tags    []string  `json:"tags"`
}

func (l Log) MarshalJSON() ([]byte, error) {
	return json.Marshal(logJSON{
		ID:      l.ID(),
		Level:   l.Level(),
		Date:    l.Date(),
		Message: l.Message(),
		Extra:   l.Extra(),
		Tags:    l.Tags(),
	})
}

func (l *Log) UnmarshalJSON(data []byte) error {
	var decodedLog logJSON

	err := json.Unmarshal(data, &decodedLog)
	if err != nil {
		return err
	}

	l.l = &log{
		id:      decodedLog.ID,
		level:   decodedLog.Level,
		date:    decodedLog.Date,
		message: decodedLog.Message,
		extra:   decodedLog.Extra,
	}
	l.tags = decodedLog.Tags

	return nil
}

// JSON returns the Log l in a json-encoded string in form of a
// slice of bytes
func (l Log) JSON() []byte {
	b, _ := json.Marshal(l)
	return b
}

func (l Log) String() string {
	return l.l.String()
}

func (l Log) Colored() string {
	return l.l.colored()
}

func (l Log) Full() string {
	return l.l.full()
}

func (l Log) FullColored() string {
	return l.l.fullColored()
}
