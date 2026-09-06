package log

import (
	"fmt"
	"os"
)

var debugEnabled = false

func SetDebug(enabled bool) {
	debugEnabled = enabled
}

func Debugf(format string, args ...any) {
	if !debugEnabled {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[D] %s\n", fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "[I] %s\n", fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "[W] %s\n", fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "[E] %s\n", fmt.Sprintf(format, args...))
}

func Fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "[F] %s\n", fmt.Sprintf(format, args...))
	os.Exit(1)
}
