package database

import (
	"context"
	"fmt"
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

type DatabaseService struct {
	pool *pgxpool.Pool
}

type DBService interface {
	Connection() *pgxpool.Pool
}

type Config struct {
	DBName  string
	Logger  Logger
	WithSSL bool
}

const adminConnStr string = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

var (
	logger  Logger
	url     string
	dbName  string
	sslMode string
)

func NewDatabase(cfg Config) (Database, error) {

	logger = cfg.Logger
	dbName = cfg.DBName
	sslMode := map[bool]string{true: "enable", false: "disable"}[cfg.WithSSL]
	url = "postgres://postgres:postgres@localhost:5432/" + dbName + "?sslmode=" + sslMode

	adminPool, err := connectAdmin()
	if err != nil {
		return nil, err
	}
	defer adminPool.Close()

	exists, err := checkDatabase(adminPool)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err = createTable(adminPool); err != nil {
			return nil, err
		}
	}

	configPool, err := pgxpool.ParseConfig(url)
	if err != nil {
		logger.LogEvent("Unable to parse database URL: " + err.Error())
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), configPool)
	if err != nil {
		logger.LogEvent("Unable to create connection pool: " + err.Error())
		return nil, err
	}

	logger.LogEvent("Connected to database")
	return &DatabaseImpl{
		pool:   pool,
		logger: logger,
	}, nil
}

// connect to system database
func connectAdmin() (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), adminConnStr)
	if err != nil {
		logger.LogEvent("Error while connecting to database: " + dbName + ". " + err.Error())
		return nil, err
	}
	return pool, nil
}

// check if database exists
func checkDatabase(pool *pgxpool.Pool) (bool, error) {

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
func createTable(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		logger.LogEvent("Error while creating database: " + dbName + ". " + err.Error())
		return err
	}
	logger.LogEvent("Database " + dbName + " created successfully")
	return nil
}

func (d *DatabaseImpl) CloseDB() {
	if d.pool != nil {
		d.pool.Close()
		d.logger.LogEvent("Database connection pool closed")
	}
}
