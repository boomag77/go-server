package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type NetListener interface {
	Accept() (net.Conn, error)
	Close() error
	Addr() net.Addr
}

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
	MaxBodyBytes   int
	UseTLS         bool
	CertFile       string
	KeyFile        string
}

type HttpServerImpl struct {
	srv      *http.Server
	logger   Logger
	mux      *http.ServeMux
	listener NetListener
	mu       sync.RWMutex
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

func validateConfig(cfg Config) error {
	if cfg.Logger == nil {
		return fmt.Errorf("logger is required")
	}
	if cfg.Port == "" {
		return fmt.Errorf("port is required")
	}
	if cfg.UseTLS && (cfg.CertFile == "" || cfg.KeyFile == "") {
		return fmt.Errorf("cert and key files are required")
	}
	return nil
}

func NewHttpServer(cfg Config) (HttpServer, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("Invalid config %w", err)
	}

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

	impl.srv = &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        mux,
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	listener, err := net.Listen("tcp", impl.srv.Addr)
	if err != nil {
		return nil, fmt.Errorf("Failed to create listener on port: %s", cfg.Port)
	}
	impl.listener = listener

	go func() {
		cfg.Logger.LogEvent("Starting server on port...: " + cfg.Port)
		var err error
		if cfg.UseTLS {
			err = impl.srv.ServeTLS(listener, cfg.CertFile, cfg.KeyFile)
		} else {
			err = impl.srv.Serve(listener)
		}
		if err != nil && err != http.ErrServerClosed {
			cfg.Logger.LogEvent("Server error: " + err.Error())
		}
	}()
	return impl, nil
}

// SetHandler sets handler for the server
func (h *HttpServerImpl) SetHandler(path string, handler http.HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logger.LogEvent("Setting handler for path: " + path)
	h.mux.HandleFunc(path, handler)
}

func (h *HttpServerImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
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
