package database

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	pool, err := InitDB()
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Create the messages table for testing
	_, err = pool.Exec(context.Background(), `
        CREATE TABLE IF NOT EXISTS messages (
            id SERIAL PRIMARY KEY,
            username TEXT NOT NULL,
            text TEXT NOT NULL
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create messages table: %v", err)
	}

	return pool
}

func teardownTestDB(pool *pgxpool.Pool) {
	// Drop the messages table after testing
	pool.Exec(context.Background(), "DROP TABLE IF EXISTS messages")
	CloseDB(pool)
}

func TestSaveMessage(t *testing.T) {
	pool := setupTestDB(t)
	defer teardownTestDB(pool)

	err := SaveMessage(context.Background(), pool, "testuser", "Hello, world!")
	assert.NoError(t, err, "Expected no error, got an error")
}

func TestGetMessages(t *testing.T) {
	pool := setupTestDB(t)
	defer teardownTestDB(pool)

	// Insert a test message
	err := SaveMessage(context.Background(), pool, "testuser", "Hello, world!")
	assert.NoError(t, err, "Expected no error, got an error")

	messages, err := GetMessages(context.Background(), pool)
	assert.NoError(t, err, "Expected no error, got an error")
	assert.Len(t, messages, 1, "Expected 1 message, got %d", len(messages))
	assert.Equal(t, "testuser", messages[0].UserName, "Expected username 'testuser', got %s", messages[0].UserName)
	assert.Equal(t, "Hello, world!", messages[0].Text, "Expected text 'Hello, world!', got %s", messages[0].Text)
}
