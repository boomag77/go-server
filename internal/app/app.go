package app

import (
	"context"
	"fmt"
	"net/http"
	"telegram_server/internal/models"
)

type Logger interface {
	LogEvent(prefix string, err error)
}

type Database interface {
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]models.Message, error)
	Ping() error
	CloseDB()
}

type Router interface {
	PingHandler(w http.ResponseWriter, r *http.Request)
	MessageHandler(w http.ResponseWriter, r *http.Request)
}

type HttpServer interface {
	SetHandler(string, http.HandlerFunc)
	Shutdown(ctx context.Context) error
}

type AppImpl struct {
	db         Database
	httpserver HttpServer
	router     Router
	bot        Bot
	logger     Logger
}

type Config struct {
	Logger     Logger
	Database   Database
	HttpServer HttpServer
	Router     Router
	Bot        Bot
}

type App interface {
	Shutdown(ctx context.Context) error
}

func NewApp(cfg Config) App {
	return &AppImpl{
		db:         cfg.Database,
		httpserver: cfg.HttpServer,
		router:     cfg.Router,
		bot:        cfg.Bot,
		logger:     cfg.Logger,
	}
}

func (a *AppImpl) Shutdown(ctx context.Context) error {
	a.logger.LogEvent("Start shutting down application...", nil)

	var errs []error

	if err := a.httpserver.Shutdown(ctx); err != nil {
		return err
	}

	a.db.CloseDB()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	a.logger.LogEvent("Application shutdown complete", nil)
	return nil

}
