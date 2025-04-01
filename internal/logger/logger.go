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
	MkDirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Executable() (string, error)
}

type OSFileSystem struct{}

func (OSFileSystem) MkDirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (OSFileSystem) Executable() (string, error) {
	return os.Executable()
}

type Logger interface {
	Start(ctx context.Context) error
	LogEvent(logString string)
	Close(ctx context.Context) error
}

type LoggerImpl struct {
	fs             FileSystem
	logger         *log.Logger
	logFileName    string
	logWorkerCount int
	bufferSize     int
	running        bool
	logChan        chan string
	wg             sync.WaitGroup
	logFile        *os.File
	mu             sync.Mutex
}

type Config struct {
	LogFileName    string
	LogWorkerCount int
	BufferSize     int
}

func defaultConfig() Config {
	return Config{
		LogFileName:    "server.log",
		LogWorkerCount: 1,
		BufferSize:     1000,
	}
}

func NewLogger(cfg Config, fs FileSystem) Logger {

	defCfg := defaultConfig()
	if cfg.LogFileName == "" {
		cfg.LogFileName = defCfg.LogFileName
	}
	if cfg.LogWorkerCount == 0 || cfg.LogWorkerCount > runtime.NumCPU() {
		cfg.LogWorkerCount = defCfg.LogWorkerCount
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = defCfg.BufferSize
	}

	return &LoggerImpl{
		fs:             fs,
		logger:         nil,
		logFileName:    cfg.LogFileName,
		logWorkerCount: cfg.LogWorkerCount,
		bufferSize:     cfg.BufferSize,
		running:        false,
		logChan:        nil,
		wg:             sync.WaitGroup{},
		logFile:        nil,
		mu:             sync.Mutex{},
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

func (l *LoggerImpl) getLogsDirectory() (string, error) {
	if dir := os.Getenv("LOGS_DIR"); dir != "" {
		return dir, nil
	}

	exePath, err := l.fs.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "logs"), nil
}

func (l *LoggerImpl) createLogsDirectory() (string, error) {
	logsDir, err := l.getLogsDirectory()
	if err != nil {
		return "", err
	}
	if err := l.fs.MkDirAll(logsDir, 0755); err != nil {
		return "", err
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
	logsDir, err := l.createLogsDirectory()
	fmt.Println(logsDir)
	if err != nil {
		return err
	}
	fileName := filepath.Join(logsDir, l.logFileName)

	// Assign to the global variable instead of shadowing it.
	l.logFile, err = l.fs.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.logger = log.New(l.logFile, "", log.LstdFlags)

	l.logChan = make(chan string, l.bufferSize)

	numWorkers := l.logWorkerCount
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
			l.logger.Println("Logger context canceled. Exiting logworker..")
			return
		}

	}
}

func (l *LoggerImpl) Close(ctx context.Context) error {
	l.mu.Lock()
	if !l.running {
		l.mu.Unlock()
		return nil
	}

	if l.logChan != nil {
		close(l.logChan)
	}
	l.running = false
	l.mu.Unlock()

	done := make(chan struct{})

	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if l.logFile != nil {
			return l.logFile.Close()
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("Logger close timeout or context canceled.")
	}
}
