package logger_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"telegram_server/config"
	"telegram_server/internal/logger"
)

func TestLogEvent(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"Normal Message", "Test log message"},
		{"Empty Message", ""},
		{"Special Characters", "!@#$%^&*()_+-=[]{};:'\",.<>/?\\"},
		{"Long Message", strings.Repeat("A", 10000)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := os.TempDir() + "/" + strings.ReplaceAll(tc.name, " ", "_") + "_log.log"
			os.Remove(tmpFile)
			config.LogFileName = tmpFile

			// Initialize logger and log the test message
			logger.Init()
			logger.LogEvent(tc.message)
			time.Sleep(100 * time.Millisecond)
			logger.Close()

			data, err := os.ReadFile(tmpFile)
			if err != nil {
				t.Fatalf("failed to read log file: %v", err)
			}
			if !strings.Contains(string(data), tc.message) {
				t.Errorf("log file does not contain %q", tc.message)
			}

			// Cleanup temporary log file
			os.Remove(tmpFile)
		})
	}
}
