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

func (l LogLevel) String() string {
	switch l {
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
}

func NewLog(level LogLevel, message string, extra string) Log {
	t := time.Now()

	return Log{
		ID: fmt.Sprintf(
			"%02d%02d%02d%02d%02d%02d%03d",
			t.Year()%100, t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), rand.Intn(1000),
		),
		Level: level, Date: t,
		Message: strings.TrimSpace(RemoveTerminalColors(message)), MessageRaw: message,
		Extra: strings.TrimSpace(RemoveTerminalColors(extra)), ExtraRaw: extra,
	}
}

// JSON returns the Log l in a json-encoded string in form of a
// slice of bytes
func (l Log) JSON() []byte {
	jsonL := struct {
		ID      string    `json:"id"`
		Level   string    `json:"level"`
		Date    time.Time `json:"date"`
		Message string    `json:"message"`
		Extra   string    `json:"extra"`
	}{
		l.ID,
		strings.TrimSpace(l.Level.String()), l.Date,
		l.Message, l.Extra,
	}

	b, _ := json.Marshal(jsonL)
	return b
}

func (l Log) String() string {
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

	return fmt.Sprintf(
		"%s[%v]%s - %s%v%s: %s",
		BRIGHT_BLACK_COLOR, l.Date.Format(TimeFormat), DEFAULT_COLOR,
		color, l.Level, DEFAULT_COLOR,
		l.MessageRaw,
	)
}

// Full is like String(), but appends all the extra information
// associated with the log instance
func (l Log) Full() string {
	if l.Extra == "" {
		return l.String()
	}

	return fmt.Sprintf(
		"[%v] - %v: %s -> %s",
		l.Date.Format(TimeFormat),
		l.Level, l.Message,
		l.Extra,
	)
}
