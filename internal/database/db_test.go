package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func setupTest(t *testing.T) func() {
	// Set up environment variable for testing
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/testdb")

	return func() {
		os.Unsetenv("DATABASE_URL")
		if DB != nil {
			CloseDB()
		}
	}
}

func TestInitDB(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	if err := InitDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	if DB == nil {
		t.Fatal("Expected DB to be initialized, but it is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := DB.Ping(ctx)
	if err != nil {
		t.Fatalf("Could not ping database: %v", err)
	}
}

func TestCloseDB(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	if err := InitDB(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	if DB == nil {
		t.Fatal("Expected DB to be initialized, but it is nil")
	}

	CloseDB()

	if DB != nil {
		t.Fatal("Expected DB to be nil after closing, but it is not")
	}
}
