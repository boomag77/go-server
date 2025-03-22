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

	logger := logger.NewLogger()
	err := logger.Start("server.log")
	if err != nil {
		fmt.Println("Error while starting logger!")
		os.Exit(1)
	}
	logger.LogEvent("Logger initialized successfully")
	defer logger.Close()

	if err := config.Init(); err != nil {
		fmt.Println("Error while initialize server configuration!")
		os.Exit(1)
	}

	a, err := app.NewApp(logger)
	if err != nil {
		logger.LogEvent("Error while creating app: " + err.Error())
		return
	}
	logger.LogEvent("App started successfully")
	defer a.Kill()

	// http.HandleFunc("/ping", a.Router.PingHandler)
	// http.HandleFunc("/message", a.Router.MessageHandler)
	// http.HandleFunc("/webhook", a.Bot.WebHookHandler)
}
