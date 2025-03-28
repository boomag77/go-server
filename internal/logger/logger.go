package logger

import (
	"context"
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
	Start(ctx context.Context) error
	LogEvent(logString string)
	Close()
}

type LoggerImpl struct {
	logger      *log.Logger
	logFileName string
	bufferSize  int
	running     bool
	logChan     chan string
	wg          sync.WaitGroup
	logFile     *os.File
	mu          sync.Mutex
}

type Config struct {
	LogFileName string
	BufferSize  int
}

func NewLogger(cfg Config) Logger {

	return &LoggerImpl{
		logger:      nil,
		logFileName: cfg.LogFileName,
		bufferSize:  cfg.BufferSize,
		running:     false,
		logChan:     nil,
		wg:          sync.WaitGroup{},
		logFile:     nil,
		mu:          sync.Mutex{},
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
func (l *LoggerImpl) Start(ctx context.Context) error {

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
	fileName := filepath.Join(logsDir, l.logFileName)

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
		go l.logWorker(ctx)
	}
	l.running = true
	return nil
}

func (l *LoggerImpl) logWorker(ctx context.Context) {
	defer l.wg.Done()

	for {
		select {
		case logString, ok := <-l.logChan:
			if !ok {
				return // channel closed
			}
			l.logger.Println(logString)
		case <-ctx.Done():
			l.logger.Println("Logger stopped.")
			return
		}

	}
}

func (l *LoggerImpl) Close() {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return
	}
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
