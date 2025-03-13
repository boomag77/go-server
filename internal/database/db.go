package database

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

var DB *pgx.Conn

func InitDB() error {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/botdb"
	}

	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
		return err
	}

	DB = conn
	log.Println("Connected to database")
	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close(context.Background())
		DB = nil
		log.Println("Database connection closed")
	}
}
