package main

import (
	"fmt"
	"os"
	"telegram_server/config"
	"telegram_server/internal/app"
	"telegram_server/internal/logger"
)

type AppInterface interface {
	Kill()
}

type Logger interface {
	Start() error
	LogEvent(string)
	Close()
}

func main() {

	appLogger := logger.NewLogger(1000)
	err := appLogger.Start("server.log")
	if err != nil {
		fmt.Println("Error while starting logger!")
		os.Exit(1)
	}
	appLogger.LogEvent("Logger initialized successfully")
	defer appLogger.Close()

	if err := config.Init(); err != nil {
		fmt.Println("Error while initialize server configuration!")
		os.Exit(1)
	}

	a, err := app.NewApp(appLogger)
	if err != nil {
		appLogger.LogEvent("Error while creating app: " + err.Error())
		return
	}
	appLogger.LogEvent("App started successfully")
	defer a.Kill()
}
