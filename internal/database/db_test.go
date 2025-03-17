package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInitDB(t *testing.T) {
	// Устанавливаем переменную окружения для тестовой БД
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	pool, err := InitDB()
	assert.NoError(t, err, "Expected no error, got an error")
	assert.NotNil(t, pool, "Expected a valid connection pool, got nil")

	// Закрываем соединение после теста
	CloseDB(pool)
}

func TestCloseDB(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	pool, err := InitDB()
	assert.NoError(t, err, "Expected no error, got an error")
	assert.NotNil(t, pool, "Expected a valid connection pool, got nil")

	// Закрываем соединение
	// Закрываем соединение
	CloseDB(pool)
	// Instead of fixed sleep, wait until pool is closed (max 500ms)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var acquireErr error
	for {
		select {
		case <-ctx.Done():
			// Timeout reached; assert that we did eventually get an error.
			assert.Error(t, acquireErr, "Expected error when acquiring a connection from a closed pool, got nil")
			return
		default:
			_, acquireErr = pool.Acquire(context.Background())
			if acquireErr != nil {
				assert.Error(t, acquireErr, "Expected error when acquiring a connection from a closed pool")
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}
