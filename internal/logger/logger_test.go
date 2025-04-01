package logger

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockFileSystem is a mock implementation of the FileSystem interface
type MockFileSystem struct {
	mock.Mock
}

func (m *MockFileSystem) MkDirAll(path string, perm os.FileMode) error {
	args := m.Called(path, perm)
	return args.Error(0)
}

func (m *MockFileSystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	args := m.Called(name, flag, perm)
	return args.Get(0).(*os.File), args.Error(1)
}

func (m *MockFileSystem) Executable() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func TestNewLogger(t *testing.T) {
	fs := &MockFileSystem{}
	cfg := Config{
		LogFileName:    "test.log",
		LogWorkerCount: 2,
		BufferSize:     500,
	}

	logger := NewLogger(cfg, fs).(*LoggerImpl)

	assert.Equal(t, "test.log", logger.logFileName)
	assert.Equal(t, 2, logger.logWorkerCount)
	assert.Equal(t, 500, logger.bufferSize)
	assert.False(t, logger.running)
	assert.Nil(t, logger.logChan)
	assert.Nil(t, logger.logFile)
}

func TestLoggerImpl_Start(t *testing.T) {
	fs := &MockFileSystem{}
	fs.On("Executable").Return("/path/to/executable", nil)
	fs.On("MkDirAll", "/path/to/logs", os.FileMode(0755)).Return(nil)

	tmpFile, err := os.CreateTemp("", "testlog")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	fs.On("OpenFile", "/path/to/logs/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0644)).Return(tmpFile, nil)

	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	ctx := context.Background()
	err = logger.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, logger.running)
	assert.NotNil(t, logger.logChan)
	assert.NotNil(t, logger.logFile)
}

func TestLoggerImpl_Start_AlreadyStarted(t *testing.T) {
	fs := &MockFileSystem{}
	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	ctx := context.Background()
	logger.running = true
	err := logger.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, "Logger already started", err.Error())
}

func TestLoggerImpl_LogEvent(t *testing.T) {
	fs := &MockFileSystem{}
	fs.On("Executable").Return("/path/to/executable", nil)
	fs.On("MkDirAll", "/path/to/logs", os.FileMode(0755)).Return(nil)

	tmpFile, err := os.CreateTemp("", "testlog")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	fs.On("OpenFile", "/path/to/logs/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0644)).Return(tmpFile, nil)

	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	ctx := context.Background()
	logger.Start(ctx)
	defer logger.Close(ctx)

	logger.LogEvent("test log")
	select {
	case log := <-logger.logChan:
		assert.Equal(t, "test log", log)
	default:
		t.Error("Expected log message not received")
	}
}

func TestLoggerImpl_LogEvent_NotStarted(t *testing.T) {
	fs := &MockFileSystem{}
	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	logger.LogEvent("test log")
}

func TestLoggerImpl_Close(t *testing.T) {
	fs := &MockFileSystem{}
	fs.On("Executable").Return("/path/to/executable", nil)
	fs.On("MkDirAll", "/path/to/logs", os.FileMode(0755)).Return(nil)

	tmpFile, err := os.CreateTemp("", "testlog")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	fs.On("OpenFile", "/path/to/logs/server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(0644)).Return(tmpFile, nil)

	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	ctx := context.Background()
	logger.Start(ctx)

	err = logger.Close(ctx)
	assert.NoError(t, err)
	assert.False(t, logger.running)
}

func TestLoggerImpl_Close_NotRunning(t *testing.T) {
	fs := &MockFileSystem{}
	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	ctx := context.Background()
	err := logger.Close(ctx)
	assert.NoError(t, err)
}

func TestLoggerImpl_Close_ContextCanceled(t *testing.T) {
	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Manually set up a logger that won't finish closing before context is canceled
	logger := &LoggerImpl{
		running: true,
		wg:      sync.WaitGroup{},
	}
	logger.wg.Add(1) // Add a wait that will never be resolved

	// Close should return an error
	err := logger.Close(ctx)
	assert.Error(t, err)
	assert.Equal(t, "Logger close timeout or context canceled.", err.Error())
}

func TestLoggerImpl_getLogsDirectory(t *testing.T) {
	fs := &MockFileSystem{}
	fs.On("Executable").Return("/path/to/executable", nil)

	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	dir, err := logger.getLogsDirectory()
	assert.NoError(t, err)
	assert.Equal(t, "/path/to/logs", dir)
}

func TestLoggerImpl_createLogsDirectory(t *testing.T) {
	fs := &MockFileSystem{}
	fs.On("Executable").Return("/path/to/executable", nil)
	fs.On("MkDirAll", "/path/to/logs", os.FileMode(0755)).Return(nil)

	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	dir, err := logger.createLogsDirectory()
	assert.NoError(t, err)
	assert.Equal(t, "/path/to/logs", dir)
}

func TestLoggerImpl_createLogsDirectory_Error(t *testing.T) {
	fs := &MockFileSystem{}
	fs.On("Executable").Return("/path/to/executable", nil)
	fs.On("MkDirAll", "/path/to/logs", os.FileMode(0755)).Return(errors.New("mkdir error"))

	cfg := defaultConfig()
	logger := NewLogger(cfg, fs).(*LoggerImpl)

	_, err := logger.createLogsDirectory()
	assert.Error(t, err)
	assert.Equal(t, "mkdir error", err.Error())
}
