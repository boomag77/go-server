package config

import (
	"os"
)

var (
	LogFileName string
	DatabaseURL string
	ServerPort  string
)

const DatabaseName = "botdb"

const defaultDBURL = "postgres://postgres:postgres@localhost:5432/" + DatabaseName + "?sslmode=disable"

const defaultServerPort = ":8080"

func Init() error {

	DatabaseURL = getEnv("DATABASE_URL", defaultDBURL)
	ServerPort = getEnv("SERVER_PORT", defaultServerPort)

	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
