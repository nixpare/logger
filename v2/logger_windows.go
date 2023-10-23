package logger

import (
	"io"
	"os"
)

func init() {
	var out io.Writer
	if _, err := os.Stdout.Stat(); err == nil {
		out = os.Stdout
	}
	DefaultLogger = NewLogger(out)
}