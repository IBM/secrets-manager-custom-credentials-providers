package utils

import (
	"fmt"
	"log"
	"strings"
)

type Logger struct {
	prefix string
}

// NewLogger initializes the logger with a list of identifiers
func NewLogger(identifiers ...string) *Logger {
	prefix := fmt.Sprintf("[%s]", strings.Join(identifiers, "]:["))
	return &Logger{prefix: prefix}
}

// Info logs an informational message
func (l *Logger) Info(message string) {
	log.Println(l.prefix, "INFO:", message)
}

// Error logs an error message
func (l *Logger) Error(err error) {
	log.Println(l.prefix, "ERROR:", err)
}
