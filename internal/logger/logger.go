package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"telegram_server/config"
)

var (
	logChan     chan string
	wg          sync.WaitGroup
	logFile     *os.File
	mu          sync.Mutex
	initialized bool
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
		// Log message successfully sent to log channel
		// Do nothing
	default:
		fmt.Println("WARNING: log channel is full, dropping log!")
	}
}

// Init initializes the logger
func Init() {

	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return
	}

	var err error

	// Assign to the global variable instead of shadowing it.
	logFile, err = os.OpenFile(config.LogFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(logFile)

	logChan = make(chan string, 1000)

	numWorkers := runtime.NumCPU()
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go logWorker()
	}

	initialized = true
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
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return
	}

	close(logChan)
	wg.Wait()
	if logFile != nil {
		logFile.Close()
	}
	initialized = false
}
