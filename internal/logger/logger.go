package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const logsFolderName = "logs"
const logFileName = "server.log"

var logFilePath string

var (
	logChan     chan string
	wg          sync.WaitGroup
	logFile     *os.File
	mu          sync.Mutex
	initialized bool
)

// Logger is a service that logs messages. Implements LogEvent method (in main.go)
type Logger interface {
	Init() error
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

func createLogsDirectory() (dir string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Unable to get work directory: %w", err)
	}

	logsDir := filepath.Join(cwd, logsFolderName)
	info, err := os.Stat(logsDir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(logsDir, os.ModePerm); err != nil {
			return "", fmt.Errorf("Failed to create logs directory: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("Failed to get info about logs directory: %w", err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", logsDir)
	}
	return logsDir, nil
}

// Init initializes the logger
func Init() error {

	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return nil
	}

	// create fileName for OpenFile
	logsDir, err := createLogsDirectory()
	if err != nil {
		return fmt.Errorf("Failed to create logs directory: %w", err)
	}
	fileName := filepath.Join(logsDir, logFileName)

	// Assign to the global variable instead of shadowing it.
	logFile, err = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Failed to open log file %s: %w", fileName, err)
	}

	log.SetOutput(logFile)

	logChan = make(chan string, 1000)

	numWorkers := runtime.NumCPU()
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go logWorker()
	}

	initialized = true

	return nil
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
