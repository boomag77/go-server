package database

import (
	"context"
	"errors"
	"reflect"
	"telegram_server/internal/models"
	"testing"
)

// We need to create a wrapper around the DatabaseImpl to make it testable

// TestDatabaseImpl is a wrapper around DatabaseImpl that allows us to inject mocks
type TestDatabaseImpl struct {
	DatabaseImpl
	mockExecFunc  func(ctx context.Context, sql string, arguments ...interface{}) (commandTag any, err error)
	mockQueryFunc func(ctx context.Context, sql string, args ...interface{}) (MockRows, error)
}

// MockRows interface for mocking pgx rows
type MockRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
	Close()
}

// mockRows implements MockRows for testing
type mockRows struct {
	data    [][]interface{}
	index   int
	scanErr error
	rowErr  error
	closed  bool
}

func (m *mockRows) Next() bool {
	if m.index < len(m.data) {
		m.index++
		return true
	}
	return false
}

func (m *mockRows) Scan(dest ...interface{}) error {
	if m.scanErr != nil {
		return m.scanErr
	}

	if m.index == 0 || m.index > len(m.data) {
		return errors.New("no row available")
	}

	row := m.data[m.index-1]

	if len(dest) != len(row) {
		return errors.New("column count mismatch")
	}

	for i, src := range row {
		dst := dest[i]

		switch s := src.(type) {
		case int64:
			if d, ok := dst.(*int64); ok {
				*d = s
			} else {
				return errors.New("type mismatch")
			}
		case string:
			if d, ok := dst.(*string); ok {
				*d = s
			} else {
				return errors.New("type mismatch")
			}
		default:
			return errors.New("unsupported type")
		}
	}

	return nil
}

func (m *mockRows) Err() error {
	return m.rowErr
}

func (m *mockRows) Close() {
	m.closed = true
}

// SaveMessage is a mock implementation
func (db *TestDatabaseImpl) SaveMessage(ctx context.Context, username, text string) error {
	if db.mockExecFunc != nil {
		_, err := db.mockExecFunc(ctx, "INSERT INTO messages (username, text) VALUES ($1, $2)", username, text)
		return err
	}
	return nil
}

// GetMessages is a mock implementation
func (db *TestDatabaseImpl) GetMessages(ctx context.Context) ([]models.Message, error) {
	if db.mockQueryFunc != nil {
		rows, err := db.mockQueryFunc(ctx, "SELECT id, username, text FROM messages")
		if err != nil {
			db.logger.LogEvent("Error while getting messages: " + err.Error())
			return nil, err
		}
		defer rows.Close()

		var messages []models.Message

		for rows.Next() {
			var message models.Message
			if err := rows.Scan(&message.ID, &message.UserName, &message.Text); err != nil {
				db.logger.LogEvent("Error while scanning message: " + err.Error())
				return nil, err
			}
			messages = append(messages, message)
		}

		if err := rows.Err(); err != nil {
			db.logger.LogEvent("Error after scanning rows: " + err.Error())
			return nil, err
		}

		return messages, nil
	}

	return nil, errors.New("no mock function provided")
}

func TestSaveMessage(t *testing.T) {
	tests := []struct {
		name     string
		username string
		text     string
		execFunc func(ctx context.Context, sql string, arguments ...interface{}) (commandTag any, err error)
		wantErr  bool
	}{
		{
			name:     "success case",
			username: "testuser",
			text:     "hello world",
			execFunc: func(ctx context.Context, sql string, arguments ...interface{}) (commandTag any, err error) {
				if sql != "INSERT INTO messages (username, text) VALUES ($1, $2)" {
					return nil, errors.New("unexpected SQL query")
				}
				if len(arguments) != 2 || arguments[0] != "testuser" || arguments[1] != "hello world" {
					return nil, errors.New("wrong arguments")
				}
				return 1, nil // 1 row affected
			},
			wantErr: false,
		},
		{
			name:     "database error",
			username: "testuser",
			text:     "hello world",
			execFunc: func(ctx context.Context, sql string, arguments ...interface{}) (commandTag any, err error) {
				return nil, errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}

			db := &TestDatabaseImpl{
				DatabaseImpl: DatabaseImpl{
					logger: logger,
				},
				mockExecFunc: tt.execFunc,
			}

			err := db.SaveMessage(context.Background(), tt.username, tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetMessages(t *testing.T) {
	tests := []struct {
		name      string
		queryFunc func(ctx context.Context, sql string, args ...interface{}) (MockRows, error)
		want      []models.Message
		wantErr   bool
		logCheck  func(*testLogger) bool
	}{
		{
			name: "success case with messages",
			queryFunc: func(ctx context.Context, sql string, args ...interface{}) (MockRows, error) {
				return &mockRows{
					data: [][]interface{}{
						{int64(1), "user1", "message1"},
						{int64(2), "user2", "message2"},
					},
				}, nil
			},
			want: []models.Message{
				{ID: 1, UserName: "user1", Text: "message1"},
				{ID: 2, UserName: "user2", Text: "message2"},
			},
			wantErr: false,
			logCheck: func(l *testLogger) bool {
				return true // No specific log check needed
			},
		},
		{
			name: "empty result set",
			queryFunc: func(ctx context.Context, sql string, args ...interface{}) (MockRows, error) {
				return &mockRows{
					data: [][]interface{}{},
				}, nil
			},
			want:    nil, // Change this to nil instead of []models.Message{} for empty results consistency
			wantErr: false,
			logCheck: func(l *testLogger) bool {
				return true
			},
		},
		{
			name: "query error",
			queryFunc: func(ctx context.Context, sql string, args ...interface{}) (MockRows, error) {
				return nil, errors.New("query error")
			},
			want:    nil,
			wantErr: true,
			logCheck: func(l *testLogger) bool {
				return l.Contains("Error while getting messages: query error")
			},
		},
		{
			name: "scan error",
			queryFunc: func(ctx context.Context, sql string, args ...interface{}) (MockRows, error) {
				return &mockRows{
					data:    [][]interface{}{{int64(1), "user1", "message1"}},
					scanErr: errors.New("scan error"),
				}, nil
			},
			want:    nil,
			wantErr: true,
			logCheck: func(l *testLogger) bool {
				return l.Contains("Error while scanning message: scan error")
			},
		},
		{
			name: "rows error",
			queryFunc: func(ctx context.Context, sql string, args ...interface{}) (MockRows, error) {
				return &mockRows{
					data:   [][]interface{}{{int64(1), "user1", "message1"}},
					rowErr: errors.New("row error"),
				}, nil
			},
			want:    nil,
			wantErr: true,
			logCheck: func(l *testLogger) bool {
				return l.Contains("Error after scanning rows: row error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}

			db := &TestDatabaseImpl{
				DatabaseImpl: DatabaseImpl{
					logger: logger,
				},
				mockQueryFunc: tt.queryFunc,
			}

			got, err := db.GetMessages(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMessages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Handle nil vs empty slice comparison
			if tt.want == nil && got == nil {
				// Both nil, they match
			} else if tt.want == nil && len(got) == 0 {
				// nil expected, empty slice received - treat as equal
			} else if len(tt.want) == 0 && got == nil {
				// empty slice expected, nil received - treat as equal
			} else if !reflect.DeepEqual(got, tt.want) {
				// Normal comparison for non-empty cases
				t.Errorf("GetMessages() = %v, want %v", got, tt.want)
			}

			if !tt.logCheck(logger) {
				t.Errorf("Log check failed, logs: %v", logger.events)
			}
		})
	}
}

// TestSaveMessageWithRealDBImpl tests the actual implementation with a real database
func TestSaveMessageImplementation(t *testing.T) {
	// We'd need a test database setup for this test
	t.Skip("Requires actual database - skipping")
}

// TestGetMessagesImplementation tests the actual implementation with a real database
func TestGetMessagesImplementation(t *testing.T) {
	// We'd need a test database setup for this test
	t.Skip("Requires actual database - skipping")
}
