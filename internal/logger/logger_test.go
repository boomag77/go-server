package logger

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestInit initializes the logger and checks that it is marked as initialized.
func TestInit(t *testing.T) {
	// Ensure the logger is closed before starting
	resetLoggerState()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Check if the logger is initialized
	mu.Lock()
	isInitialized := initialized
	mu.Unlock()
	if !isInitialized {
		t.Error("Logger should be initialized but is not")
	}

	// Clean up
	Close()
}

// TestInitReentrant verifies that calling Init() multiple times does not cause errors.
func TestInitReentrant(t *testing.T) {
	// Reset to a known state
	resetLoggerState()

	// Call Init() twice
	err := Init()
	if err != nil {
		t.Fatalf("First Init() call failed: %v", err)
	}

	err = Init()
	if err != nil {
		t.Fatalf("Second Init() call failed: %v", err)
	}

	// Clean up
	Close()
}

// TestLogEvent verifies that a log message is written to the log file.
func TestLogEvent(t *testing.T) {
	// Сбрасываем состояние логгера
	resetLoggerState()

	// Создаём временный каталог для теста
	tempDir, cleanup := makeTempDir(t)
	defer cleanup()

	// Сохраняем текущую рабочую директорию и переключаемся на tempDir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Не удалось получить текущую директорию: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Не удалось перейти в tempDir: %v", err)
	}
	// Восстанавливаем рабочую директорию после завершения теста
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			t.Fatalf("Не удалось восстановить рабочую директорию: %v", err)
		}
	}()

	// В данном тесте оставляем исходные значения:
	// logsFolderName = "logs", logFileName = "server.log"
	// Поэтому функция createLogsDirectory создаст путь tempDir/logs

	// Инициализируем логгер
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Логируем тестовое сообщение
	testMessage := "test log message"
	LogEvent(testMessage)

	// Ждём, чтобы логгер обработал запись
	time.Sleep(200 * time.Millisecond)

	// Закрываем логгер, чтобы все данные были записаны в файл
	Close()

	// Формируем путь до файла логов, который должен быть создан в tempDir/logs
	logFilePath := filepath.Join(tempDir, "logs", "server.log")
	verifyLogContains(t, logFilePath, testMessage)
}

// TestConcurrentLogEvent checks if the logger handles concurrent access properly.
func TestConcurrentLogEvent(t *testing.T) {
	// Reset to a known state
	resetLoggerState()

	// Use a temporary directory so we don't pollute the working directory
	tempDir, cleanup := makeTempDir(t)
	defer cleanup()

	// Override package-level constants for tests
	oldLogsFolderName := logsFolderName
	oldLogFileName := logFileName
	logsFolderName = filepath.Base(tempDir)
	logFileName = "conc_test.log"
	defer func() {
		logsFolderName = oldLogsFolderName
		logFileName = oldLogFileName
	}()

	// Temporarily change working directory to the parent of tempDir
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working dir: %v", err)
	}
	if err = os.Chdir(filepath.Dir(tempDir)); err != nil {
		t.Fatalf("Failed to chdir to parent of tempDir: %v", err)
	}
	defer os.Chdir(oldWd)

	// Initialize the logger
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	const (
		numGoroutines        = 5
		messagesPerGoroutine = 50
	)

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				LogEvent(t.Name() + ":" + "goroutine" + string('0'+rune(id)) + "_" + string('0'+rune(j)))
			}
		}(i)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Give the logger time to flush
	time.Sleep(200 * time.Millisecond)
	Close()

	// Verify that log entries were written
	logFilePath := filepath.Join(tempDir, "conc_test.log")
	file, err := os.Open(logFilePath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	linesFound := 0
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), t.Name()) {
			linesFound++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Error scanning log file: %v", err)
	}

	expected := numGoroutines * messagesPerGoroutine
	if linesFound < expected {
		t.Errorf("Expected at least %d lines from concurrent logs, but found %d", expected, linesFound)
	}
}

// TestClose checks that the logger closes gracefully.
func TestClose(t *testing.T) {
	// Reset to a known state
	resetLoggerState()

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	Close()

	mu.Lock()
	isInitialized := initialized
	mu.Unlock()

	if isInitialized {
		t.Error("Logger should be marked as closed (initialized=false), but it's still true")
	}
}

// TestCreateLogsDirectory checks the directory creation logic.
func TestCreateLogsDirectory(t *testing.T) {
	// Keep current working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Make a temp directory to emulate a new workspace
	tempDir, cleanup := makeTempDir(t)
	defer cleanup()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to chdir to tempDir: %v", err)
	}
	defer os.Chdir(origWd)

	// Override logs folder name for the test
	oldLogsFolderName := logsFolderName
	logsFolderName = "test_logs"
	defer func() { logsFolderName = oldLogsFolderName }()

	dir, err := createLogsDirectory()
	if err != nil {
		t.Fatalf("createLogsDirectory() returned error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Failed to stat created directory: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", dir)
	}
}

// resetLoggerState is a helper to put the logger in a known state before each test.
func resetLoggerState() {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		// If logger was initialized, close it
		if logChan != nil {
			close(logChan)
		}
		wg.Wait()
		if logFile != nil {
			logFile.Close()
		}
	}

	logFile = nil
	initialized = false
}

// makeTempDir creates a temporary directory and returns the directory path
// along with a cleanup function that removes the directory.
func makeTempDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	cleanup := func() {
		os.RemoveAll(dir)
	}
	return dir, cleanup
}

// verifyLogContains checks if fileName contains the expected string.
func verifyLogContains(t *testing.T, fileName, expected string) {
	t.Helper()

	f, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("Failed to open log file %s: %v", fileName, err)
	}
	defer f.Close()

	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), expected) {
			found = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	if !found {
		t.Errorf("Expected log file %s to contain '%s', but it was not found", fileName, expected)
	}
}
