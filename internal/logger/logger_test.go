package logger_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"telegram_server/config"
	"telegram_server/internal/logger"
)

func TestLogEvent(t *testing.T) {
	// Setup temporary log file
	tmpFile := os.TempDir() + "/test_log.log"
	os.Remove(tmpFile)
	config.LogFileName = tmpFile

	// Initialize logger and log a test message
	logger.Init()
	testMessage := "Test log message"
	logger.LogEvent(testMessage)
	// Allow worker to process the log event
	time.Sleep(100 * time.Millisecond)
	logger.Close()

	// Read the log file and verify the test message is present
	data, err := ioutil.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(data), testMessage) {
		t.Errorf("log file does not contain test message")
	}

	// Cleanup temporary log file
	os.Remove(tmpFile)
}
