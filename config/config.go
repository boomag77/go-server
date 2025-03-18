package config

import (
	"os"
)

var (
	LogFileName string
	DatabaseURL string
	ServerPort string
)

func Init() {
	LogFileName = getEnv("LOG_FILE_NAME", "server.log")
	DatabaseURL = getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/botdb")
	ServerPort = getEnv("SERVER_PORT", ":8080")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
