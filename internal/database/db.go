package database

import (
	"context"
	"fmt"
	"os"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger = contracts.Logger
type Database = contracts.Database
type Message = models.Message
type LogMessage = contracts.LogMessage

type DatabaseImpl struct {
	configPool *pgxpool.Config
	pool       *pgxpool.Pool
	logger     Logger
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
		if dbName := os.Getenv("DB_NAME"); dbName != "" {
			cfg.DBName = dbName
		} else {
			cfg.DBName = def.DBName
		}
	}
	if cfg.Host == "" {
		if host := os.Getenv("DB_HOST"); host != "" {
			cfg.Host = host
		} else {
			cfg.Host = def.Host
		}
	}
	if cfg.Port == 0 {
		if port := os.Getenv("DB_PORT"); port != "" {
			fmt.Sscanf(port, "%d", &cfg.Port)
		} else {
			cfg.Port = def.Port
		}
	}
	if cfg.User == "" {
		if user := os.Getenv("DB_USER"); user != "" {
			cfg.User = user
		} else {
			cfg.User = def.User
		}
	}
	if cfg.AllowAutocreate == nil {
		cfg.AllowAutocreate = def.AllowAutocreate
	}
	if cfg.Password == "" {
		if password := os.Getenv("DB_PASSWORD"); password != "" {
			cfg.Password = password
		} else {
			cfg.Password = def.Password
		}
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
		return nil, fmt.Errorf("logger is required. Cannot create database")
	}

	cfg = applyDefaultsIfNotSet(cfg)

	url := createURL(cfg, "user")

	exists, err := isExists(cfg)
	if err != nil {
		return nil, err
	}
	if !exists {
		if *cfg.AllowAutocreate {
			err := createDB(cfg)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("database does not exist and autocreate is disabled")
		}
	}

	configPool, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %w", err)
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
		return fmt.Errorf("no database connection")
	}
	return d.pool.Ping(context.Background())
}

func (d *DatabaseImpl) Connect(ctx context.Context) error {
	var err error
	d.pool, err = pgxpool.NewWithConfig(ctx, d.configPool)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}

	d.logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.DatabaseName,
		Message: "Connected to database",
		Err:     nil,
	})
	_, err = d.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS messages (
		id SERIAL PRIMARY KEY,
		username TEXT NOT NULL,
		text TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error while creating non-existing table: %w", err)
	}
	return nil
}

func createURL(config Config, accessType string) string {
	dbName := map[string]string{"admin": "postgres", "user": config.DBName}[accessType]
	sslMode := map[bool]string{true: "enable", false: "disable"}[config.WithSSL]
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		dbName,
		sslMode,
	)
}

// connect to system database
func connectAdmin(config Config) (*pgxpool.Pool, error) {
	adminConnStr := createURL(config, "admin")
	fmt.Println("Connecting to admin database -> " + adminConnStr)
	adminPool, err := pgxpool.New(context.Background(), adminConnStr)
	if err != nil {
		return nil, fmt.Errorf("error while getting adminPool at system Database")
	}
	return adminPool, nil
}

// check if database exists
func isExists(config Config) (bool, error) {

	adminPool, err := connectAdmin(config)
	if err != nil {
		return false, err
	}
	defer adminPool.Close()

	var exists bool
	err = adminPool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", config.DBName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error while checking if database %s exists: %w", config.DBName, err)
	}
	return exists, nil
}

// create table if it does not exist
func createDB(config Config) error {

	adminPool, err := connectAdmin(config)
	if err != nil {
		return err
	}
	defer adminPool.Close()

	_, err = adminPool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", config.DBName))
	if err != nil {
		return fmt.Errorf("error while creating database: %w", err)
	}
	config.Logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.DatabaseName,
		Message: "Database " + config.DBName + " created successfully",
		Err:     nil,
	})
	return nil
}

func (d *DatabaseImpl) Disconnect() {
	if d.pool != nil {
		d.pool.Close()
		d.logger.LogEvent(LogMessage{
			Level:   models.LevelInfo,
			Service: contracts.DatabaseName,
			Message: "Database connection pool closed, disconnected succesfully",
			Err:     nil,
		})
	}
}
