package logger

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

const (
	DEFAULT_COLOR = "\x1b[0m"
	BLACK_COLOR = "\x1b[30m"
	DARK_RED_COLOR = "\x1b[31m"
	DARK_GREEN_COLOR = "\x1b[32m"
	DARK_YELLOW_COLOR = "\x1b[33m"
	DARK_BLUE_COLOR = "\x1b[34m"
	DARK_MAGENTA_COLOR = "\x1b[35m"
	DARK_CYAN_COLOR = "\x1b[36m"
	DARK_WHITE_COLOR = "\x1b[37m"
	BRIGHT_BLACK_COLOR = "\x1b[90m"
	BRIGHT_RED_COLOR = "\x1b[31m"
	BRIGHT_GREEN_COLOR = "\x1b[32m"
	BRIGHT_YELLOW_COLOR = "\x1b[33m"
	BRIGHT_BLUE_COLOR = "\x1b[34m"
	BRIGHT_MAGENTA_COLOR = "\x1b[35m"
	BRIGHT_CYAN_COLOR = "\x1b[36m"
	WHITE_COLOR = "\x1b[37m"
)

var all_terminal_colors = [...]string{ DEFAULT_COLOR, BLACK_COLOR, DARK_RED_COLOR, DARK_GREEN_COLOR, DARK_YELLOW_COLOR,
								DARK_BLUE_COLOR, DARK_MAGENTA_COLOR, DARK_CYAN_COLOR, DARK_WHITE_COLOR, BRIGHT_BLACK_COLOR,
								BRIGHT_RED_COLOR, BRIGHT_GREEN_COLOR, BRIGHT_YELLOW_COLOR, BRIGHT_BLUE_COLOR,
								BRIGHT_MAGENTA_COLOR, BRIGHT_CYAN_COLOR, WHITE_COLOR }

func RemoveTerminalColors(s string) string {
	for _, x := range all_terminal_colors {
		s = strings.ReplaceAll(s, x, "")
	}
	return s
}

func ToTerminal(out io.Writer) bool {
	switch out := out.(type) {
	case *os.File:
		stat, _ := out.Stat()
    	return (stat.Mode() & os.ModeCharDevice) == os.ModeCharDevice
	default:
		return false
	}
}

// IndentString takes a string and indents every line with
// the provided number of single spaces
func IndentString(s string, n int) string {
	split := strings.Split(s, "\n")
	var res string

	for _, line := range split {
		for i := 0; i < n; i++ {
			res += " "
		}
		res += line + "\n"
	}

	return strings.TrimRight(res, " \n")
}

func LogsToJSON(logs []Log) []byte {
	b, err := json.Marshal(logs)
	if err != nil {
		panic(err)
	}

	return b
}

func LogsToJSONIndented(logs []Log, spaces int) []byte {
	indent := ""
	for i := 0; i < spaces; i++ {
		indent += " "
	}

	b, err := json.MarshalIndent(logs, "", indent)
	if err != nil {
		panic(err)
	}

	return b
}

func Fatal(a ...any) {
	DefaultLogger.Print(LOG_LEVEL_FATAL, a...)
	os.Exit(1)
}

func Fatalf(format string, a ...any) {
	DefaultLogger.Printf(LOG_LEVEL_FATAL, format, a...)
	os.Exit(1)
}

func LogsMatch(logs []Log, tags ...string) []Log {
	lMatch := make([]Log, 0)
	for _, log := range logs {
		if log.Match(tags...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}

func LogsMatchAny(logs []Log, tags ...string) []Log {
	lMatch := make([]Log, 0)
	for _, log := range logs {
		if log.MatchAny(tags...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}

func LogsLevelMatch(logs []Log, levels ...LogLevel) []Log {
	lMatch := make([]Log, 0)
	for _, log := range logs {
		if log.LevelMatchAny(levels...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}