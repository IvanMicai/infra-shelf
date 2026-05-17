// Package output is the CLI's user-facing logging surface: ANSI-colored
// success/error/info/warn lines. Writes to stdout (success/info/warn/title/dim)
// and stderr (error).
package output

import (
	"fmt"
	"io"
	"os"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"
	ansiRed   = "\x1b[31m"
	ansiGreen = "\x1b[32m"
	ansiCyan  = "\x1b[36m"
)

var (
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

func wrap(color, s string) string {
	if !colorEnabled() {
		return s
	}
	return color + s + ansiReset
}

func colorEnabled() bool {
	// Respect NO_COLOR (https://no-color.org/) and disable when not a TTY.
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := Stdout.(*os.File)
	if !ok {
		return true
	}
	stat, err := f.Stat()
	if err != nil {
		return true
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func Success(msg string) { fmt.Fprintln(Stdout, wrap(ansiGreen, "✔ "+msg)) }
func Error(msg string)   { fmt.Fprintln(Stderr, wrap(ansiRed, "✘ "+msg)) }
func Info(msg string)    { fmt.Fprintln(Stdout, wrap(ansiCyan, "ℹ "+msg)) }
func Warn(msg string)    { fmt.Fprintln(Stdout, "⚠ "+msg) }
func Title(msg string)   { fmt.Fprintln(Stdout, wrap(ansiBold, msg)) }
func Dim(msg string)     { fmt.Fprintln(Stdout, wrap(ansiDim, msg)) }

func Successf(format string, args ...any) { Success(fmt.Sprintf(format, args...)) }
func Errorf(format string, args ...any)   { Error(fmt.Sprintf(format, args...)) }
func Infof(format string, args ...any)    { Info(fmt.Sprintf(format, args...)) }
func Warnf(format string, args ...any)    { Warn(fmt.Sprintf(format, args...)) }
