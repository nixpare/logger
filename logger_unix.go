//go:build !windows
package logger

import (
	"os"
)

func init() {
	DefaultLogger = NewLogger(os.Stdout)
}