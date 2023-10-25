package logger

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var (
	TimeFormat = "2006-01-02 15:04:05.00" // TimeFormat defines which timestamp to use with the logs
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
	log_level_stdout
	log_level_stderr
)

func (level LogLevel) String() string {
	switch level {
	case LOG_LEVEL_BLANK, log_level_stdout, log_level_stderr:
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
	switch level {
	case log_level_stdout:
		return json.Marshal("stdout")
	case log_level_stderr:
		return json.Marshal("stderr")
	default:
		return json.Marshal(strings.TrimSpace(strings.ToLower(level.String())))
	}
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
	case "stdout":
		*level = log_level_stdout
	case "stderr":
		*level = log_level_stderr
	default:
		*level = -1
	}

	return nil
}

type log struct {
	id      string
	level   LogLevel
	date    time.Time
	message string
	extra   string
}

func (l log) cleanMessage() string {
	return strings.TrimSpace(RemoveTerminalColors(l.message))
}

func (l log) cleanExtra() string {
	return strings.TrimSpace(RemoveTerminalColors(l.extra))
}

func newLog(level LogLevel, message string, extra string) *log {
	t := time.Now()

	if level == log_level_stdout || level == log_level_stderr {
		message = message + " " + extra
		extra = ""
	}

	return &log{
		id: fmt.Sprintf(
			"%d%03d",
			t.UnixNano() / 1000, rand.Intn(1000),
		),
		level: level, date: t,
		message: message, extra: extra,
	}
}

func (l log) String() string {
	switch l.level {
	case LOG_LEVEL_BLANK:
		return fmt.Sprintf(
			"[%v] - %s",
			l.date.Format(TimeFormat),
			l.cleanMessage(),
		)
	case log_level_stdout, log_level_stderr:
		return l.cleanMessage()
	default:
		return fmt.Sprintf(
			"[%v] - %v: %s",
			l.date.Format(TimeFormat),
			l.level, l.cleanMessage(),
		)
	}
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
	case LOG_LEVEL_ERROR, log_level_stderr:
		color = DARK_RED_COLOR
	case LOG_LEVEL_FATAL:
		color = BRIGHT_RED_COLOR
	}

	switch l.level {
	case LOG_LEVEL_BLANK:
		return fmt.Sprintf(
			"%s[%v]%s - %s",
			BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
			l.message,
		)
	case log_level_stdout:
		return l.message
	case log_level_stderr:
		return fmt.Sprintf(
			"%s%s%s",
			DARK_RED_COLOR, l.message, DEFAULT_COLOR,
		)
	default:
		return fmt.Sprintf(
			"%s[%v]%s - %s%v%s: %s",
			BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
			color, l.level, DEFAULT_COLOR,
			l.message,
		)
	}
}

func (l log) full() string {
	if l.extra == "" {
		// log_level_stdout and log_level_stderr always in this case
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

func (l log) fullColored() string {
	if l.extra == "" {
		// log_level_stdout and log_level_stderr always in this case
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
			"%s[%v]%s - %s\n%s",
			BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
			l.message, IndentString(l.extra, 4),
		)
	}

	return fmt.Sprintf(
		"%s[%v]%s - %s%v%s: %s\n%s",
		BRIGHT_BLACK_COLOR, l.date.Format(TimeFormat), DEFAULT_COLOR,
		color, l.level, DEFAULT_COLOR,
		l.message, IndentString(l.extra, 4),
	)
}

// Log holds every information. It keeps the error severity level (see the constants),
// the time it was created at and the message associated with it. 
// It also has the optional field "extra" that can be used to store additional information:
// Automatically everything after the first new line will be stored there. By default it
// will be displayed with an indentation, but you can hide it by calling the Logger method
// DisableExtras()
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

// RawMessage returns the logger message (as the Message() method) unmodified:
// if the Logger output is a terminal, the logger will automatically decorate
// the message with terminal colors. This method gives you back not only the message
// but also every terminal character for the color-handling
func (l Log) RawMessage() string {
	return l.l.message
}

func (l Log) Extra() string {
	return l.l.cleanExtra()
}

// RawExtra returns the logger extra information (as the Extra() method) unmodified:
// see the method RawMessage() for other informations.
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

// Match returns true if the Log has every tag you
// have provided, otherwise returns false
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

// MatchAny returns true if the Log has at least one of
// the tags you have provided, otherwise returns false
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

// LevelMatchAny returns true if the Log has one of the
// log levels you have provided, otherwise returns false
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

// JSON returns the Log in a json-encoded string
func (l Log) JSON() []byte {
	b, _ := json.Marshal(l)
	return b
}

func (l Log) String() string {
	return l.l.String()
}

// Colored returns the message with the terminal decorations
func (l Log) Colored() string {
	return l.l.colored()
}

// Full return the message and the extras together
func (l Log) Full() string {
	return l.l.full()
}

// FullColored return the message and the extras together with
// the terminal decorations
func (l Log) FullColored() string {
	return l.l.fullColored()
}
