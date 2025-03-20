package database

import (
	"context"
	"fmt"
	"telegram_server/config"
	"telegram_server/internal/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

const adminConnStr string = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

var Pool *pgxpool.Pool

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

// create table if it does not exist
func createTable(dbName string, pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		logger.LogEvent("Error while creating database: " + dbName + ". " + err.Error())
		return err
	}
	logger.LogEvent("Database " + dbName + " created successfully")
	return nil
}

func InitDB() error {

	adminPool, err := connectAdmin()
	if err != nil {
		return err
	}
	defer adminPool.Close()

	exists, err := checkDatabase(config.DatabaseName, adminPool)
	if err != nil {
		return err
	}
	if !exists {
		if err = createTable(config.DatabaseName, adminPool); err != nil {
			return err
		}
	}

	connStr := config.DatabaseURL

	configPool, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		logger.LogEvent("Unable to parse database URL: " + err.Error())
		return err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), configPool)
	if err != nil {
		logger.LogEvent("Unable to create connection pool: " + err.Error())
		return err
	}

	Pool = pool
	logger.LogEvent("Connected to database")
	return nil
}

func CloseDB() {
	if Pool != nil {
		Pool.Close()
		logger.LogEvent("Database connection pool closed")
	}
}
