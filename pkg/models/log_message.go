package models

import (
	"strings"
	"unicode"
)

type LogLevel string

const (
	LevelInfo    LogLevel = "INFO"
	LevelWarning LogLevel = "WARNING"
	LevelError   LogLevel = "ERROR"
)

// LogMessage represents a log message with a level and an error
type LogMessage struct {
	Level   LogLevel // Log level (e.g., "INFO", "WARNING", "ERROR")
	Service string   // Service name
	Message string
	Err     error
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	firstRune := runes[0]
	var b strings.Builder
	b.Grow(len(s))
	if unicode.IsLetter(firstRune) && unicode.IsUpper(firstRune) {
		b.WriteRune(firstRune)
		b.WriteString(strings.ToLower(string(runes[1:])))
		return b.String()
	}

	b.WriteRune(unicode.ToUpper(firstRune))
	b.WriteString(strings.ToLower(string(runes[1:])))
	return b.String()
}

func (lm LogMessage) Formatted() string {

	var builder strings.Builder

	if lm.Err == nil {
		builder.Grow(
			len(lm.formattedLevel()) +
				len(lm.formattedService()) +
				len(lm.Message))

		builder.WriteString(lm.formattedLevel())
		builder.WriteString(lm.formattedService())
		builder.WriteString(capitalize(lm.Message))
	} else {
		builder.Grow(len(lm.formattedLevel()) + len(lm.formattedService()) + len(lm.Message) + 2 + len(lm.sanitizedError()))

		builder.WriteString(lm.formattedLevel())
		builder.WriteString(lm.formattedService())
		builder.WriteString(capitalize(lm.Message))
		builder.WriteString(lm.sanitizedError())
	}

	return builder.String()
}

func (lm LogMessage) sanitizedError() string {
	var msg string
	defer func() {
		if r := recover(); r != nil {
			msg = "[error formatting failed]"
		}
	}()

	msg = lm.Err.Error()
	if strings.Contains(msg, "password") {
		return "[sanitized: password]"
	}
	if strings.Contains(msg, "token") {
		return "[sanitized: token]"
	}
	return msg
}

func (lm LogMessage) formattedLevel() string {
	switch lm.Level {
	case LevelInfo:
		return "[INFO]    "
	case LevelWarning:
		return "[WARNING] "
	case LevelError:
		return "[ERROR]   "
	default:
		return "[UNKNOWN] "
	}
}

func (lm LogMessage) formattedService() string {
	if lm.Service == "" {
		return "UNKNOWN SERVICE -> "
	}
	var b strings.Builder
	b.Grow(len(lm.Service) + 4)
	b.WriteString(strings.ToUpper(lm.Service))
	b.WriteString(" -> ")
	return b.String() // "SERVICE_NAME -> "
}
