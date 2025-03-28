package database

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testLogger is a simple implementation for testing that records log events.
type testLogger struct {
	events []string
}

func (l *testLogger) LogEvent(event string) {
	l.events = append(l.events, event)
}

func (l *testLogger) Contains(substr string) bool {
	for _, e := range l.events {
		if strings.Contains(e, substr) {
			return true
		}
	}
	return false
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.DBName != "botdb" {
		t.Errorf("expected DBName 'botdb', got %s", cfg.DBName)
	}
	if cfg.Host != "localhost" {
		t.Errorf("expected Host 'localhost', got %s", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected Port 5432, got %d", cfg.Port)
	}
	if cfg.User != "postgres" {
		t.Errorf("expected User 'postgres', got %s", cfg.User)
	}
	if cfg.Password != "postgres" {
		t.Errorf("expected Password 'postgres', got %s", cfg.Password)
	}
	if cfg.AllowAutocreate == nil {
		t.Errorf("expected AllowAutocreate to be non-nil")
	} else if *cfg.AllowAutocreate != true {
		t.Errorf("expected AllowAutocreate true, got %v", *cfg.AllowAutocreate)
	}
	if cfg.WithSSL != false {
		t.Errorf("expected WithSSL false, got %v", cfg.WithSSL)
	}
	if cfg.MaxConns != 5 {
		t.Errorf("expected MaxConns 5, got %d", cfg.MaxConns)
	}
	if cfg.MaxConnLifetime != 15*time.Minute {
		t.Errorf("expected MaxConnLifetime 15m, got %v", cfg.MaxConnLifetime)
	}
}

func TestApplyDefaultsIfNotSet(t *testing.T) {
	input := Config{
		DBName:          "",
		Host:            "",
		Port:            0,
		User:            "",
		Password:        "",
		AllowAutocreate: nil, // false should be replaced by default (true)
		MaxConns:        0,
		MaxConnLifetime: 0,
	}
	result := applyDefaultsIfNotSet(input)
	def := defaultConfig()
	if result.DBName != def.DBName {
		t.Errorf("expected DBName %s, got %s", def.DBName, result.DBName)
	}
	if result.Host != def.Host {
		t.Errorf("expected Host %s, got %s", def.Host, result.Host)
	}
	if result.Port != def.Port {
		t.Errorf("expected Port %d, got %d", def.Port, result.Port)
	}
	if result.User != def.User {
		t.Errorf("expected User %s, got %s", def.User, result.User)
	}
	if result.Password != def.Password {
		t.Errorf("expected Password %s, got %s", def.Password, result.Password)
	}
	if result.AllowAutocreate == nil {
		t.Errorf("expected AllowAutocreate to be non-nil after applying defaults")
	} else if def.AllowAutocreate == nil {
		t.Errorf("unexpected nil AllowAutocreate in default config")
	} else if *result.AllowAutocreate != *def.AllowAutocreate {
		t.Errorf("expected AllowAutocreate %v, got %v", *def.AllowAutocreate, *result.AllowAutocreate)
	}
	if result.MaxConns != def.MaxConns {
		t.Errorf("expected MaxConns %d, got %d", def.MaxConns, result.MaxConns)
	}
	if result.MaxConnLifetime != def.MaxConnLifetime {
		t.Errorf("expected MaxConnLifetime %v, got %v", def.MaxConnLifetime, result.MaxConnLifetime)
	}
}

func TestCreateURL(t *testing.T) {
	cfg := Config{
		User:     "user",
		Password: "pass",
		Host:     "127.0.0.1",
		Port:     1234,
		DBName:   "testdb",
		WithSSL:  true,
	}
	expected := "postgres://user:pass@127.0.0.1:1234/testdb?sslmode=enable"
	url := createURL(cfg)
	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}

	cfg.WithSSL = false
	expected = "postgres://user:pass@127.0.0.1:1234/testdb?sslmode=disable"
	url = createURL(cfg)
	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}
}

func TestNewDatabase_LoggerNil(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = nil
	_, err := NewDatabase(cfg)
	if err == nil || !strings.Contains(err.Error(), "Logger is required") {
		t.Errorf("expected Logger is required error, got %v", err)
	}
}

func TestNewDatabase_InvalidURL(t *testing.T) {
	// Use an invalid port (such as -1) so that pgxpool.ParseConfig fails.
	logger := &testLogger{}
	cfg := defaultConfig()
	cfg.Logger = logger
	cfg.Port = -1
	_, err := NewDatabase(cfg)
	if err == nil {
		t.Errorf("expected error due to invalid port in URL, got nil")
	}
}

func TestNewDatabase_NotExists_AutoCreateDisabled(t *testing.T) {
	// Using a random DB name that likely does not exist.
	logger := &testLogger{}
	cfg := defaultConfig()
	cfg.Logger = logger
	cfg.DBName = "nonexistent_db_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	allowAutocreate := false
	cfg.AllowAutocreate = &allowAutocreate
	_, err := NewDatabase(cfg)
	if err == nil || !strings.Contains(err.Error(), "Database does not exist") {
		t.Errorf("expected non-existent database error with autocreate disabled, got %v", err)
	}
}

func TestPing_NoConnection(t *testing.T) {
	logger := &testLogger{}
	dbImpl := &DatabaseImpl{
		pool:   nil,
		logger: logger,
	}
	err := dbImpl.Ping()
	if err == nil || err.Error() != "No database connection." {
		t.Errorf("expected error 'No database connection.', got %v", err)
	}
}

func TestConnect_Failure(t *testing.T) {
	logger := &testLogger{}
	cfg := defaultConfig()
	cfg.Logger = logger
	// Use an invalid configuration (invalid port) so that Connect fails.
	cfg.Port = -1
	db, err := NewDatabase(cfg)
	if err == nil {
		err = db.Connect(context.Background())
		if err == nil {
			db.CloseDB()
			t.Errorf("expected Connect to fail with invalid configuration")
		}
	}
}

func TestCloseDB_NoPool(t *testing.T) {
	logger := &testLogger{}
	dbImpl := &DatabaseImpl{
		pool:   nil,
		logger: logger,
	}
	// Calling CloseDB with a nil pool should not panic.
	dbImpl.CloseDB()
}
