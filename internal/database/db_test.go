package database

import (
	"context"
	"os"
	"testing"

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
	CloseDB(pool)

	// Проверяем, что соединение закрыто
	_, err = pool.Acquire(context.Background())
	assert.Error(t, err, "Expected error when acquiring a connection from a closed pool, got nil")
}
