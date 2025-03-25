package server

import (
	"context"
	"net/http"
	"telegram_server/config"
	"time"
)

type Logger interface {
	LogEvent(string)
	Close()
}

type Config struct {
	Port   string
	Logger Logger
	routes map[string]http.HandlerFunc
}

type HttpServerImpl struct {
	srv    *http.Server
	logger Logger
}

type HttpServer interface {
	SetHandler(string, http.HandlerFunc)
	Shutdown(context.Context) error
}

func NewHttpServer(cfg Config) (HttpServer, error) {
	server := &http.Server{
		Addr: ":8080",
	}
	errChan := make(chan error, 1)

	go func() {
		cfg.Logger.LogEvent("Starting server on port... " + config.ServerPort)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	select {
	case err := <-errChan:
		return nil, err
	case <-time.After(100 * time.Millisecond):
		cfg.Logger.LogEvent("Server is running on port " + cfg.Port)
		return &HttpServerImpl{
			srv:    server,
			logger: cfg.Logger,
		}, nil
	}
}

// SetHandler sets handler for the server
func (h *HttpServerImpl) SetHandler(path string, handler http.HandlerFunc) {
	http.HandleFunc(path, handler)
}

// shutdown server
func (h *HttpServerImpl) Shutdown(ctx context.Context) error {
	h.logger.LogEvent("Shutting down server...")

	if err := h.srv.Shutdown(ctx); err != nil {
		h.logger.LogEvent("Error while shutting down server: " + err.Error())
		return err
	}

	h.logger.LogEvent("Server is down!")
	return nil
}
