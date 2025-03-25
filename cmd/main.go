package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"telegram_server/config"
	"telegram_server/internal/app"
	"telegram_server/internal/bot"
	"telegram_server/internal/database"
	"telegram_server/internal/logger"
	"telegram_server/internal/router"
	"telegram_server/internal/server"
	"time"
)

type App interface {
	Start() error
	Shutdown(ctx context.Context) error
}

type Logger interface {
	Start(logFileName string) error
	LogEvent(string)
	Close()
}

func main() {

	config.Init()

	ctx := context.Background()

	appLogger := logger.NewLogger(1000)
	err := appLogger.Start("server.log")
	if err != nil {
		fmt.Println("Error while starting logger!")
		os.Exit(1)
	}
	defer appLogger.Close()
	appLogger.LogEvent("Logger initialized successfully")

	db, err := database.NewDatabase(appLogger)
	if err != nil {
		appLogger.LogEvent("Failed to connect to database: " + err.Error())
		os.Exit(1)
	}

	config := server.Config{
		Port:   ":8080",
		Logger: appLogger,
	}

	httpSrv, err := server.NewHttpServer(config)
	if err != nil {
		appLogger.LogEvent("Failed to start server: " + err.Error())
		db.CloseDB()
		os.Exit(1)
	}

	newRouter, err := router.NewRouter(appLogger, db)
	if err != nil {
		db.CloseDB()
		appLogger.LogEvent("Failed to create router: " + err.Error())
		os.Exit(1)
	}

	newBot, err := bot.NewBot(appLogger, db)
	if err != nil {
		db.CloseDB()
		appLogger.LogEvent("Failed to create bot: " + err.Error())
	}

	httpSrv.SetHandler("/ping", newRouter.PingHandler)
	httpSrv.SetHandler("/message", newRouter.MessageHandler)

	cfg := app.Config{
		Logger:     appLogger,
		Database:   db,
		HttpServer: httpSrv,
		Router:     newRouter,
		Bot:        newBot,
	}

	application := app.NewApp(cfg)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})

	go func() {
		sig := <-sigChan
		appLogger.LogEvent("Received signal: " + sig.String())

		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := application.Shutdown(shutdownCtx); err != nil {
			appLogger.LogEvent("Error while shutting down application: " + err.Error())
			os.Exit(1)
		}
		close(done)
	}()

	<-done
	appLogger.LogEvent("Application's shutted down successfully")
}
