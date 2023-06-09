//go:build !windows
package logger

import (
	"io"
	"os"
)

func init() {
	DefaultLogger = NewLogger(os.Stdout)
}