package logger

import (
	"encoding/json"
	"io"
	"os"
	"strings"
)

// This colors can be used to customize the output. You can achieve this
// by simply doing:
/*
	fmt.Printf("%s%s%s", logger.DARK_RED_COLOR, "My string", "logger.DEFAULT_COLOR")
*/
// Remember to always use the DEFAULT_COLOR to reset the terminal to the default
// color. You can check if the output is a terminal window or not with the
// ToTerminal function.
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

// RemoveTerminalColors strips every terminal color provided from this package
// from a string
func RemoveTerminalColors(s string) string {
	for _, x := range all_terminal_colors {
		s = strings.ReplaceAll(s, x, "")
	}
	return s
}

// ToTerminal tests if out is a terminal window or not
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

// LogsToJSON converts a slice of logs in JSON format
func LogsToJSON(logs []Log) []byte {
	b, err := json.Marshal(logs)
	if err != nil {
		panic(err)
	}

	return b
}

// LogsToJSON converts a slice of logs in JSON format with
// the provided indentation length in spaces, not tabs
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

// Fatal creates a fatal log via the DefaultLogger and calls os.Exit(1)
func Fatal(a ...any) {
	DefaultLogger.Print(LOG_LEVEL_FATAL, a...)
	os.Exit(1)
}

// Fatald creates a fatal log via the DefaultLogger and calls os.Exit(1)
func Fatalf(format string, a ...any) {
	DefaultLogger.Printf(LOG_LEVEL_FATAL, format, a...)
	os.Exit(1)
}

// LogsMatch returns the logs that match every tag provided
func LogsMatch(logs []Log, tags ...string) []Log {
	lMatch := make([]Log, 0)
	for _, log := range logs {
		if log.Match(tags...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}

// LogsMatchAny returns the logs that match at least one of the tag provided
func LogsMatchAny(logs []Log, tags ...string) []Log {
	lMatch := make([]Log, 0)
	for _, log := range logs {
		if log.MatchAny(tags...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}

// LogsLevelMatch returns the logs that match one of the severity levels provided
func LogsLevelMatch(logs []Log, levels ...LogLevel) []Log {
	lMatch := make([]Log, 0)
	for _, log := range logs {
		if log.LevelMatchAny(levels...) {
			lMatch = append(lMatch, log)
		}
	}
	return lMatch
}
