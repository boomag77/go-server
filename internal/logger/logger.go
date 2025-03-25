package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type FileSystem interface {
	CreateDirectory(path string) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
}

type Logger interface {
	Start(logFileName string) error
	LogEvent(logString string)
	Close()
}

type LoggerImpl struct {
	logger     *log.Logger
	bufferSize int
	running    bool
	logChan    chan string
	wg         sync.WaitGroup
	logFile    *os.File
	mu         sync.Mutex
}

// NewLogger creates a new LoggerImpl

func NewLogger(bufferSize int) Logger {

	return &LoggerImpl{
		logger:     nil,
		bufferSize: bufferSize,
		running:    false,
		logChan:    nil,
		wg:         sync.WaitGroup{},
		logFile:    nil,
		mu:         sync.Mutex{},
	}
}

// LogEvent logs a message
func (l *LoggerImpl) LogEvent(logString string) {
	l.mu.Lock()
	running := l.running
	logChan := l.logChan
	l.mu.Unlock()

	if !running || logChan == nil {
		fmt.Println("WARNING: logger not started, dropping log!")
		return
	}

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

	logsDir := filepath.Join(cwd, "logs")
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
func (l *LoggerImpl) Start(logFileName string) error {

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return fmt.Errorf("Logger already started")
	}

	// create fileName for OpenFile
	logsDir, err := createLogsDirectory()
	if err != nil {
		return err
	}
	fileName := filepath.Join(logsDir, logFileName)

	// Assign to the global variable instead of shadowing it.
	l.logFile, err = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.logger = log.New(l.logFile, "", log.LstdFlags)

	l.logChan = make(chan string, l.bufferSize)

	numWorkers := runtime.NumCPU()
	for i := 1; i <= numWorkers; i++ {
		l.wg.Add(1)
		go l.logWorker()
	}
	l.running = true
	return nil
}

func (l *LoggerImpl) logWorker() {
	defer l.wg.Done()

	l.mu.Lock()
	logger := l.logger
	l.mu.Unlock()

	for {
		logString, ok := <-l.logChan
		if !ok {
			return
		}
		logger.Println(logString)
	}
}

func (l *LoggerImpl) Close() {
	l.mu.Lock()
	if l.logChan != nil {
		close(l.logChan)
	}
	l.mu.Unlock()

	// Ждем завершения воркеров без удержания мьютекса
	l.wg.Wait()

	// Закрываем файл
	l.mu.Lock()
	if l.logFile != nil {
		l.logFile.Close()
	}
	l.mu.Unlock()
}
