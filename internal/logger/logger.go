package logger

import (
	"log"
	"os"
	"runtime"
	"sync"
	"telegram_server/config"
)

var (
	logChan chan string
	wg      sync.WaitGroup
	logFile *os.File
)

// Logger is a service that logs messages. Implements LogEvent method (in main.go)
type Logger interface {
	Init()
	LogEvent(logString string)
	Close()
}

// LogEvent logs a message
func LogEvent(logString string) {
	select {
	case logChan <- logString:

	default:
		log.Println("WARNING: log channel is full, dropping log!")
	}
}

// Init initializes the logger
func Init() {
	var err error

	// Assign to the global variable instead of shadowing it.
	logFile, err = os.OpenFile(config.LogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(logFile)

	logChan = make(chan string, 100)

	numWorkers := runtime.NumCPU()
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go logWorker()
	}
}

func logWorker() {
	defer wg.Done()
	for {
		logString, ok := <-logChan
		if !ok {
			return
		}
		log.Println(logString)
	}
}

func Close() {
	close(logChan)
	wg.Wait()
	if logFile != nil {
		logFile.Close()
	}
}
