package server

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Logger interface {
	LogEvent(string)
	Close()
}

type Config struct {
	Port           string
	Logger         Logger
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	Handler        http.Handler
}

type HttpServerImpl struct {
	srv    *http.Server
	logger Logger
	mux    *http.ServeMux
}

type HttpServer interface {
	SetHandler(string, http.HandlerFunc)
	Shutdown(context.Context) error
}

func defaultConfig() Config {
	return Config{
		Port:           "8080",
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

func NewHttpServer(cfg Config) (HttpServer, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	defCfg := defaultConfig()

	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defCfg.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defCfg.WriteTimeout
	}
	if cfg.MaxHeaderBytes == 0 {
		cfg.MaxHeaderBytes = defCfg.MaxHeaderBytes
	}

	mux := http.NewServeMux()

	impl := &HttpServerImpl{
		logger: cfg.Logger,
		mux:    mux,
	}

	server := &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        mux,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	errChan := make(chan error, 1)
	started := make(chan struct{})

	go func() {
		cfg.Logger.LogEvent("Starting server on port... " + cfg.Port)
		close(started)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return nil, err
	case <-started:
		cfg.Logger.LogEvent("Server is running on port " + cfg.Port)
		return impl, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("Error starting server (timeout)")
	}
}

// SetHandler sets handler for the server
func (h *HttpServerImpl) SetHandler(path string, handler http.HandlerFunc) {
	h.mux.HandleFunc(path, handler)
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
