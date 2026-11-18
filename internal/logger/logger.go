// Package logger provides the plugin log output.
package logger

import (
	"log"
	"os"
)

// Logger writes to stdout, which Traefik includes in its own log output.
type Logger struct {
	out   *log.Logger
	debug bool
}

// New returns a logger whose lines carry the middleware name.
func New(name string, debug bool) *Logger {
	return &Logger{out: log.New(os.Stdout, "[matomo-tracking "+name+"] ", 0), debug: debug}
}

// Debugf logs only when debug output is enabled in the configuration.
func (l *Logger) Debugf(format string, args ...any) {
	if l.debug {
		l.out.Printf("DEBUG "+format, args...)
	}
}

// Errorf logs a failure.
func (l *Logger) Errorf(format string, args ...any) {
	l.out.Printf("ERROR "+format, args...)
}
