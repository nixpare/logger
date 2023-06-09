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
	LOG_LEVEL_BLANK = iota
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

// Log is the structure that can be will store any log reported
// with Logger. It keeps the error severity level (see the constants)
// the date it was created and the message associated with it (probably
// an error). It also has the optional field "extra" that can be used to
// store additional information
type Log struct {
	ID         string    `json:"id"`
	Level      LogLevel  `json:"level"`  // Level is the Log severity (INFO - DEBUG - WARNING - ERROR - FATAL)
	Date       time.Time `json:"date"`   // Date is the timestamp of the log creation
	Message    string    `json:"message"`// Message is the main message that should summarize the event
	MessageRaw string    `json:"-"`
	Extra      string    `json:"extra"`  // Extra should hold any extra information provided for deeper understanding of the event
	ExtraRaw   string    `json:"-"`
	Tags       []string  `json:"tags,omitempty"`
}

func NewLog(level LogLevel, message string, extra string, tags ...string) *Log {
	t := time.Now()

	return &Log{
		ID: fmt.Sprintf(
			"%02d%02d%02d%02d%02d%02d%03d",
			t.Year()%100, t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), rand.Intn(1000),
		),
		Level: level, Date: t,
		Message: strings.TrimSpace(RemoveTerminalColors(message)), MessageRaw: message,
		Extra: strings.TrimSpace(RemoveTerminalColors(extra)), ExtraRaw: extra,
		Tags: tags,
	}
}

// JSON returns the Log l in a json-encoded string in form of a
// slice of bytes
func (l Log) JSON() []byte {
	b, _ := json.Marshal(l)
	return b
}

func (l Log) String() string {
	if l.Level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"[%v] - %s",
			l.Date.Format(TimeFormat),
			l.Message,
		)
	}

	return fmt.Sprintf(
		"[%v] - %v: %s",
		l.Date.Format(TimeFormat),
		l.Level, l.Message,
	)
}

func (l Log) Colored() string {
	var color string
	switch l.Level {
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

	if l.Level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"%s[%v]%s - %s%s",
			BRIGHT_BLACK_COLOR, l.Date.Format(TimeFormat), DEFAULT_COLOR,
			l.MessageRaw, DEFAULT_COLOR,
		)
	}

	return fmt.Sprintf(
		"%s[%v]%s - %s%v%s: %s%s",
		BRIGHT_BLACK_COLOR, l.Date.Format(TimeFormat), DEFAULT_COLOR,
		color, l.Level, DEFAULT_COLOR,
		l.MessageRaw, DEFAULT_COLOR,
	)
}

// Full is like String(), but appends all the extra information
// associated with the log instance
func (l Log) Full() string {
	if l.Extra == "" {
		return l.String()
	}

	if l.Level == LOG_LEVEL_BLANK {
		return fmt.Sprintf(
			"[%v] - %s -> %s",
			l.Date.Format(TimeFormat),
			l.Message, l.Extra,
		)
	}

	return fmt.Sprintf(
		"[%v] - %v: %s -> %s",
		l.Date.Format(TimeFormat),
		l.Level, l.Message,
		l.Extra,
	)
}

func (l *Log) AddTags(tags ...string) {
	loop: for _, tag := range tags {
		tag = strings.ToLower(tag)
		
		for _, lTags := range l.Tags {
			if tag == lTags {
				continue loop
			}
		}
		
		l.Tags = append(l.Tags, tag)
	}
}

func (l Log) Match(tags ...string) bool {
	for _, matchTag := range tags {
		var hasMatch bool
		for _, logTag := range l.Tags {
			if strings.ToLower(matchTag) == logTag {
				hasMatch = true
				break
			}
		}
		if (!hasMatch) {
			return false
		}
	}
	return true
}

func (l Log) MatchAny(tags ...string) bool {
	for _, matchTag := range tags {
		for _, logTag := range l.Tags {
			if strings.ToLower(matchTag) == logTag {
				return true
			}
		}
	}
	return false
}

func (l Log) LevelMatchAny(levels ...LogLevel) bool {
	for _, level := range levels {
		if l.Level == level {
			return true
		}
	}
	return false
}
