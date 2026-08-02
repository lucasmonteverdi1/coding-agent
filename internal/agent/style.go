package agent

import (
	"os"
)

// ANSI styling for the REPL. Deliberately hand-rolled: the REPL is built on
// fmt.Print and a line-reading bufio.Reader, and a TUI framework would need to
// own the terminal loop to earn its keep.
const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
	ansiBold  = "\033[1m"
)

// colorEnabled is resolved once at startup: honour NO_COLOR, and skip escapes
// when stdout is redirected to a file or pipe.
var colorEnabled = detectColor()

func detectColor() bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func dim(s string) string {
	if !colorEnabled {
		return s
	}
	return ansiDim + s + ansiReset
}

func bold(s string) string {
	if !colorEnabled {
		return s
	}
	return ansiBold + s + ansiReset
}
