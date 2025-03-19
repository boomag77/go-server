package config

import (
	"fmt"
	"os"
)

var (
	LogFileName string
	DatabaseURL string
	ServerPort  string
)

const DatabaseName = "botdb"
const defaultLogFileName = "server.log"
var defaultDBURL = fmt.Sprintf("postgres://postgres:postgres@localhost:5432/%s?sslmode=disable", DatabaseName)
const defaultServerPort = ":8080"

func Init() {
	LogFileName = getEnv("LOG_FILE_NAME", defaultLogFileName)
	DatabaseURL = getEnv("DATABASE_URL", defaultDBURL)
	ServerPort = getEnv("SERVER_PORT", defaultServerPort)
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
