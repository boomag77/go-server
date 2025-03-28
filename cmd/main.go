package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"telegram_server/internal/app"
	"telegram_server/internal/bot"
	"telegram_server/internal/database"
	"telegram_server/internal/logger"
	"telegram_server/internal/models"
	"telegram_server/internal/router"
	"telegram_server/internal/server"
	"time"
)

type App interface {
	Start() error
	Shutdown(ctx context.Context) error
}

type Logger interface {
	Start() error
	LogEvent(string)
	Close()
}

type Database interface {
	Connect() error
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]models.Message, error)
	CloseDB()
}

func main() {

	ctx := context.Background()

	loggerConfig := logger.Config{
		BufferSize:  1000,
		LogFileName: "server.log",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	appLogger := logger.NewLogger(loggerConfig)
	err := appLogger.Start(ctx)
	if err != nil {
		fmt.Println("Error while starting logger!")
		os.Exit(1)
	}
	defer appLogger.Close()
	appLogger.LogEvent("Logger initialized successfully")

	dbConfig := database.Config{
		DBName:  "botdb",
		Logger:  appLogger,
		WithSSL: false,
	}

	db, err := database.NewDatabase(dbConfig)
	if err != nil {
		appLogger.LogEvent("Failed to connect to database: " + err.Error())
		os.Exit(1)
	}

	err = db.Connect(context.Background())
	if err != nil {
		appLogger.LogEvent("Failed to connect to database: " + err.Error())
		os.Exit(1)
	}

	srvConfig := server.Config{
		Port:   "8080",
		Logger: appLogger,
	}

	httpSrv, err := server.NewHttpServer(srvConfig)
	if err != nil {
		appLogger.LogEvent("Failed to create the server: " + err.Error())
		db.CloseDB()
		os.Exit(1)
	}

	if err := httpSrv.Start(); err != nil {
		appLogger.LogEvent("Failed to start the server: " + err.Error())
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
		appLogger.LogEvent("Failed to create bot: " + err.Error())
		newBot = nil
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
