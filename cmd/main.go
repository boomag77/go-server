package main

import (
	"fmt"
	"net/http"
	"os"
	"telegram_server/config"
	"telegram_server/internal/database"
	"telegram_server/internal/logger"
	"telegram_server/internal/server"
)

func main() {
	if err := config.Init(); err != nil {
		fmt.Println("Error while initialize server configuration!")
		os.Exit(1)
	}

	if err := logger.Init(); err != nil {
		fmt.Println("Error while initialize logging service! " + err.Error())
		os.Exit(1)
	} else {
		logger.LogEvent("Logger initialized successfully")
		defer logger.Close()
	}

	err := database.InitDB()
	if err != nil {
		logger.LogEvent("WARNING!!! ---> Error while initializing database: " + err.Error())
		return
	}
	defer database.CloseDB()

	http.HandleFunc("/ping", server.PingHandler)
	http.HandleFunc("/message", server.MessageHandler)
	http.HandleFunc("/webhook", server.WebHookHandler)

	if err := server.Start(); err != nil {
		logger.LogEvent("Error while starting server: " + err.Error())
		return
	}
	server.Shutdown()
}
