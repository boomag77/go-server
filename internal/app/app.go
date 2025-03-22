package app

import (
	"fmt"
	"net/http"
	"telegram_server/internal/bot"
	"telegram_server/internal/database"
	"telegram_server/internal/router"
	"telegram_server/internal/server"
)

type Logger interface {
	LogEvent(string)
}

type Database interface {
	CloseDB()
}

type Router interface {
	PingHandler(w http.ResponseWriter, r *http.Request)
	MessageHandler(w http.ResponseWriter, r *http.Request)
}

type HttpServer interface {
	SetHandler(string, http.HandlerFunc)
	Shutdown()
}

type Bot interface {
	SendMessage(chatID int64, text string) error
	WebHookHandler(w http.ResponseWriter, r *http.Request)
}

type AppImpl struct {
	db         Database
	httpserver HttpServer
	Router     Router
	Bot        Bot
}

type App interface {
	Kill()
}

func NewApp(logger Logger) (App, error) {
	newDatabase, err := database.NewDatabase(logger)
	if err != nil {
		return nil, fmt.Errorf("Error while creating database: %w", err)
	}
	newRouter, err := router.NewRouter(logger, newDatabase)
	if err != nil {
		return nil, fmt.Errorf("Error while creating router: %w", err)
	}

	newBot, err := bot.NewBot(logger, newDatabase)
	if err != nil {
		return nil, fmt.Errorf("Error while creating bot: %w", err)
	}
	httpSrv, err := server.NewHttpServer(logger)
	if err != nil {
		return nil, fmt.Errorf("Error while starting server: %w", err)
	}

	httpSrv.SetHandler("/ping", newRouter.PingHandler)
	httpSrv.SetHandler("/message", newRouter.MessageHandler)

	return &AppImpl{
		db:         newDatabase,
		httpserver: httpSrv,
		Router:     newRouter,
		Bot:        newBot,
	}, nil
}

func (a *AppImpl) Kill() {
	a.httpserver.Shutdown()
	a.db.CloseDB()

}
