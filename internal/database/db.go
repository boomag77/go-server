package database

import (
	"context"
	"fmt"
	"telegram_server/config"
	"telegram_server/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger interface {
	LogEvent(string)
}

type DatabaseImpl struct {
	pool   *pgxpool.Pool
	logger Logger
}

type Database interface {
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]models.Message, error)
	CloseDB()
}

const adminConnStr string = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

func NewDatabase(l Logger) (Database, error) {
	adminPool, err := connectAdmin(l)
	if err != nil {
		return nil, err
	}
	defer adminPool.Close()

	exists, err := checkDatabase(config.DatabaseName, adminPool, l)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err = createTable(config.DatabaseName, adminPool, l); err != nil {
			return nil, err
		}
	}

	connStr := config.DatabaseURL

	configPool, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		l.LogEvent("Unable to parse database URL: " + err.Error())
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), configPool)
	if err != nil {
		l.LogEvent("Unable to create connection pool: " + err.Error())
		return nil, err
	}

	l.LogEvent("Connected to database")
	return &DatabaseImpl{
		pool:   pool,
		logger: l,
	}, nil
}

// connect to system database
func connectAdmin(l Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), adminConnStr)
	if err != nil {
		l.LogEvent("Error while connecting to database: " + config.DatabaseName + ". " + err.Error())
		return nil, err
	}
	return pool, nil
}

// check if database exists
func checkDatabase(dbName string, pool *pgxpool.Pool, l Logger) (bool, error) {

	var exists bool
	err := pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		l.LogEvent("Error while checking if database " + dbName + "exists. " + err.Error())
		return false, err
	}
	return exists, nil
}

// create table if it does not exist
func createTable(dbName string, pool *pgxpool.Pool, l Logger) error {
	_, err := pool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		l.LogEvent("Error while creating database: " + dbName + ". " + err.Error())
		return err
	}
	l.LogEvent("Database " + dbName + " created successfully")
	return nil
}

func (d *DatabaseImpl) CloseDB() {
	if d.pool != nil {
		d.pool.Close()
		d.logger.LogEvent("Database connection pool closed")
	}
}
