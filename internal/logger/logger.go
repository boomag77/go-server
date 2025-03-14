package logger

import (
	"log"
)

const logFileName string = "server.log"

// Logger is a service that logs messages. Implements LogEvent method (in main.go)
type Logger interface {
	LogEvent(message string)
}

// LogEvent logs a message
func LogEvent(message string) {
	log.Println(message)
}
