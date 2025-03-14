package database

import (
	"context"
	"log"
	"telegram_server/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB() (*pgxpool.Pool, error) {
	connStr := config.DatabaseURL

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("Unable to parse database URL: %v\n", err)
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Printf("Unable to create connection pool: %v\n", err)
		return nil, err
	}

	if err = migrateDB(pool); err != nil {
		CloseDB(pool)
		return nil, err
	}

	log.Println("Connected to database")
	return pool, nil
}

func CloseDB(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
		log.Println("Database connection pool closed")
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
		log.Printf("Error while creating table: %v\n", err)
		return err
	}

	log.Println("Table created successfully")
	return nil
}
