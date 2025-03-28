package database

import (
	"context"
	"fmt"
	"telegram_server/internal/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger interface {
	LogEvent(string)
}

type DatabaseImpl struct {
	configPool *pgxpool.Config
	pool       *pgxpool.Pool
	logger     Logger
}

type Database interface {
	Connect(ctx context.Context) error
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]models.Message, error)
	Ping() error
	CloseDB()
}

type Config struct {
	DBName          string
	Host            string
	Port            int
	User            string
	Password        string
	AllowAutocreate *bool
	Logger          Logger
	WithSSL         bool
	MaxConns        int
	MaxConnLifetime time.Duration
}

func defaultConfig() Config {
	defaultDBName := "botdb"
	defaultHost := "localhost"
	defaultPort := 5432
	defaultUser := "postgres"
	defaultPassword := "postgres"
	allowAutocreate := true
	defaultWithSSL := false
	defaultMaxConns := 5
	defaultMaxConnLifetime := 15 * time.Minute

	return Config{
		DBName:          defaultDBName,
		Host:            defaultHost,
		Port:            defaultPort,
		User:            defaultUser,
		Password:        defaultPassword,
		AllowAutocreate: &allowAutocreate,
		WithSSL:         defaultWithSSL,
		MaxConns:        defaultMaxConns,
		MaxConnLifetime: defaultMaxConnLifetime,
	}
}

func applyDefaultsIfNotSet(cfg Config) Config {
	def := defaultConfig()
	if cfg.DBName == "" {
		cfg.DBName = def.DBName
	}
	if cfg.Host == "" {
		cfg.Host = def.Host
	}
	if cfg.Port == 0 {
		cfg.Port = def.Port
	}
	if cfg.User == "" {
		cfg.User = def.User
	}
	if cfg.AllowAutocreate == nil {
		cfg.AllowAutocreate = def.AllowAutocreate
	}
	if cfg.Password == "" {
		cfg.Password = def.Password
	}
	if cfg.MaxConns == 0 {
		cfg.MaxConns = def.MaxConns
	}
	if cfg.MaxConnLifetime == 0 {
		cfg.MaxConnLifetime = def.MaxConnLifetime
	}
	return cfg
}

func NewDatabase(cfg Config) (Database, error) {

	if cfg.Logger == nil {
		return nil, fmt.Errorf("Logger is required. Cannot create database.")
	}

	cfg = applyDefaultsIfNotSet(cfg)

	url := createURL(cfg)

	exists, err := isExists(cfg.DBName, cfg.Logger)
	if err != nil {
		return nil, err
	}
	if !exists {
		if *cfg.AllowAutocreate {
			err := createDB(cfg.DBName, cfg.Logger)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("Database does not exist and autocreate is disabled.")
		}
	}

	configPool, err := pgxpool.ParseConfig(url)
	if err != nil {
		cfg.Logger.LogEvent("Unable to parse database URL")
		return nil, err
	}

	configPool.MaxConns = int32(cfg.MaxConns)
	configPool.MaxConnLifetime = cfg.MaxConnLifetime

	return &DatabaseImpl{
		pool:       nil,
		configPool: configPool,
		logger:     cfg.Logger,
	}, nil
}

func (d *DatabaseImpl) Ping() error {
	if d.pool == nil {
		return fmt.Errorf("No database connection.")
	}
	return d.pool.Ping(context.Background())
}

func (d *DatabaseImpl) Connect(ctx context.Context) error {
	var err error
	d.pool, err = pgxpool.NewWithConfig(ctx, d.configPool)
	if err != nil {
		d.logger.LogEvent("Unable to connect to database.")
		return err
	}

	d.logger.LogEvent("Connected to database")
	return nil
}

func createURL(config Config) string {
	sslMode := map[bool]string{true: "enable", false: "disable"}[config.WithSSL]
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
		sslMode,
	)
}

// connect to system database
func connectAdmin() (*pgxpool.Pool, error) {
	const adminConnStr string = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	adminPool, err := pgxpool.New(context.Background(), adminConnStr)
	if err != nil {
		return nil, fmt.Errorf("Error while getting adminPool at system Database.")
	}
	return adminPool, nil
}

// check if database exists
func isExists(dbName string, logger Logger) (bool, error) {

	adminPool, err := connectAdmin()
	if err != nil {
		return false, err
	}
	defer adminPool.Close()

	var exists bool
	err = adminPool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		logger.LogEvent("Error while checking if database " + dbName + "exists. ")
		return false, err
	}
	return exists, nil
}

// create table if it does not exist
func createDB(newDBName string, logger Logger) error {

	adminPool, err := connectAdmin()
	if err != nil {
		return err
	}
	defer adminPool.Close()

	_, err = adminPool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", newDBName))
	if err != nil {
		logger.LogEvent("Error while creating database: " + newDBName + ".")
		return err
	}
	logger.LogEvent("Database " + newDBName + " created successfully")
	return nil
}

func (d *DatabaseImpl) CloseDB() {
	if d.pool != nil {
		d.pool.Close()
		d.logger.LogEvent("Database connection pool closed")
	}
}
