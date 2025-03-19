package database

import (
	"context"
	"fmt"
	"telegram_server/config"
	"telegram_server/internal/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

const adminConnStr string = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

// connect to system database
func connectAdmin() (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), adminConnStr)
	if err != nil {
		logger.LogEvent("Error while connecting to database: " + config.DatabaseName + ". " + err.Error())
		return nil, err
	}
	return pool, nil
}

// check if database exists
func checkDatabase(dbName string, pool *pgxpool.Pool) (bool, error) {

	var exists bool
	err := pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		logger.LogEvent("Error while checking if database " + dbName + "exists. " + err.Error())
		return false, err
	}
	return exists, nil
}

// create table if not exists
func createTable(dbName string, pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		logger.LogEvent("Error while creating database: " + dbName + ". " + err.Error())
		return err
	}
	logger.LogEvent("Database " + dbName + " created successfully")
	return nil
}

func InitDB() (*pgxpool.Pool, error) {

	adminPool, err := connectAdmin()
	if err != nil {
		return nil, err
	}
	defer adminPool.Close()

	exists, err := checkDatabase(config.DatabaseName, adminPool)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err = createTable(config.DatabaseName, adminPool); err != nil {
			return nil, err
		}
	}

	// Use the environment variable if it exists, otherwise fall back to config.DatabaseURL.
	connStr := config.DatabaseURL

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
