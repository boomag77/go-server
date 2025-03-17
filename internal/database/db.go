package database

import (
	"context"
	"os"
	"telegram_server/config"
	"telegram_server/internal/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB() (*pgxpool.Pool, error) {
	// Use the environment variable if it exists, otherwise fall back to config.DatabaseURL.
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = config.DatabaseURL
	}

	configPool, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		logger.LogEvent("Unable to parse database URL: " + err.Error())
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), configPool)
	if err != nil {
		logger.LogEvent("Unable to create connection pool: " + err.Error())
		return nil, err
	}

	if err = migrateDB(pool); err != nil {
		CloseDB(pool)
		return nil, err
	}

	logger.LogEvent("Connected to database")
	return pool, nil
}

func CloseDB(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
		logger.LogEvent("Database connection pool closed")
	}
}

// migrate if table not exists
func migrateDB(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			text TEXT NOT NULL
		)
	`)
	if err != nil {
		logger.LogEvent("Error while creating table: " + err.Error())
		return err
	}

	logger.LogEvent("Table created successfully")
	return nil
}
