//go:build !windows
package logger

import (
	"os"
)

func init() {
	DefaultLogger = newMemLogger(os.Stdout, nil)
}