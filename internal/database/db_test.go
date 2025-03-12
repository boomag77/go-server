package database

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// Mock DB variable
var DB *sql.DB

// Mock InitDB function
func InitDB() {
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
}

// Mock CloseDB function
func CloseDB() {
	if DB != nil {
		DB.Close()
		DB = nil
	}
}

func TestInitDB(t *testing.T) {
	// Set up environment variable for testing
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	InitDB()

	if DB == nil {
		t.Fatal("Expected DB to be initialized, but it is nil")
	}

	// Clean up
	CloseDB()
}

func TestCloseDB(t *testing.T) {
	// Set up environment variable for testing
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	InitDB()

	if DB == nil {
		t.Fatal("Expected DB to be initialized, but it is nil")
	}

	CloseDB()

	if DB != nil {
		t.Fatal("Expected DB to be nil after closing, but it is not")
	}
}
