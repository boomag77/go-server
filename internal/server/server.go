package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"
	"time"
)

type Logger = contracts.Logger
type LogMessage = contracts.LogMessage
type NetListener = contracts.NetListener
type HttpServer = contracts.HttpServer

type Config struct {
	Logger         Logger
	Port           string
	CertFile       string
	KeyFile        string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxHeaderBytes int
	MaxBodyBytes   int
	UseTLS         bool
}

type HttpServerImpl struct {
	logger   Logger
	listener NetListener
	mu       sync.RWMutex
	certFile string
	keyFile  string
	srv      *http.Server
	mux      *http.ServeMux
	useTLS   bool
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

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Базовые заголовки безопасности
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Проверка размера тела
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

		next.ServeHTTP(w, r)
	})
}

func NewHttpServer(cfg Config) (HttpServer, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config %w", err)
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
		logger:   cfg.Logger,
		mux:      mux,
		useTLS:   cfg.UseTLS,
		certFile: cfg.CertFile,
		keyFile:  cfg.KeyFile,
	}

	impl.srv = &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        securityMiddleware(mux),
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}

	return impl, nil
}

func (h *HttpServerImpl) Start(ctx context.Context) error {
	if h.srv == nil {
		return fmt.Errorf("server is not initialized")
	}

	listener, err := net.Listen("tcp", h.srv.Addr)
	if err != nil {
		return fmt.Errorf("failed to create listener on port: %s", h.srv.Addr)
	}
	h.listener = listener

	go func() {
		h.logger.LogEvent(LogMessage{
			Level:   models.LevelInfo,
			Service: contracts.HTTPServerName,
			Message: "Starting server on port: " + h.srv.Addr,
			Err:     nil,
		})
		var err error
		if h.useTLS {
			err = h.srv.ServeTLS(h.listener, h.certFile, h.keyFile)
		} else {
			err = h.srv.Serve(listener)
		}
		if err != nil && err != http.ErrServerClosed {
			h.logger.LogEvent(LogMessage{
				Level:   models.LevelError,
				Service: contracts.HTTPServerName,
				Message: "Error starting server",
				Err:     err,
			})
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := h.srv.Shutdown(shutdownCtx)
		if err != nil {
			h.logger.LogEvent(LogMessage{
				Level:   models.LevelError,
				Service: contracts.HTTPServerName,
				Message: "Error shutting down server",
				Err:     err,
			})
		} else {
			h.logger.LogEvent(LogMessage{
				Level:   models.LevelInfo,
				Service: contracts.HTTPServerName,
				Message: "Server shutdown complete",
				Err:     nil,
			})
		}
	}()

	return nil
}

// SetHandler sets handler for the server
func (h *HttpServerImpl) SetHandler(path string, handler http.HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.HTTPServerName,
		Message: "Setting handler for path: " + path,
		Err:     nil,
	})
	h.mux.HandleFunc(path, handler)
}

func (h *HttpServerImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// shutdown server
func (h *HttpServerImpl) Shutdown(ctx context.Context) error {
	h.logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.HTTPServerName,
		Message: "Shutting down server...",
		Err:     nil,
	})

	if err := h.srv.Shutdown(ctx); err != nil {
		h.logger.LogEvent(LogMessage{
			Level:   models.LevelError,
			Service: contracts.HTTPServerName,
			Message: "Error while shutting down server",
			Err:     err,
		})
		return err
	}

	h.logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.HTTPServerName,
		Message: "Server is down!",
		Err:     nil,
	})
	return nil
}
