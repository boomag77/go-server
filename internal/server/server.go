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
type LogLevel = models.LogLevel

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
	config   Config
	listener NetListener
	mu       sync.RWMutex
	certFile string
	keyFile  string
	srv      *http.Server
	mux      *http.ServeMux
	useTLS   bool
}

func (h *HttpServerImpl) log(level LogLevel, message string, err error) {
	h.config.Logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   level,
			Service: contracts.HTTPServerName,
			Message: message,
			Err:     err,
		})
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
		config:   cfg,
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

func (h *HttpServerImpl) IsHealthy() bool {
	return true
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
		h.log(models.LevelInfo, "Starting server on port: "+h.srv.Addr, nil)
		var err error
		if h.useTLS {
			err = h.srv.ServeTLS(h.listener, h.certFile, h.keyFile)
		} else {
			err = h.srv.Serve(listener)
		}
		if err != nil && err != http.ErrServerClosed {
			h.log(models.LevelError, "Error starting server", err)

		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := h.srv.Shutdown(shutdownCtx)
		if err != nil {
			h.log(models.LevelError, "Error shutting down server", err)
		} else {
			h.log(models.LevelInfo, "Server shutdown complete", nil)

		}
	}()

	return nil
}

// SetHandler sets handler for the server
func (h *HttpServerImpl) SetHandler(path string, handler http.HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.log(models.LevelInfo, "Setting handler for path: "+path, nil)
	h.mux.HandleFunc(path, handler)
	h.log(models.LevelInfo, "Handler set for path: "+path, nil)
}

func (h *HttpServerImpl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// shutdown server
func (h *HttpServerImpl) Shutdown(ctx context.Context) error {

	if h.srv == nil {
		h.log(models.LevelWarning, "Trying to shutdown server, but Server is not initialized", nil)
		return nil
	}

	h.log(models.LevelInfo, "Starting shutting down server", nil)

	err := h.srv.Shutdown(ctx)
	if err != nil {
		h.log(models.LevelError, "Error shutting down server", err)
		return err
	}
	h.log(models.LevelInfo, "Server shutdown complete", nil)

	return nil

}

func (h *HttpServerImpl) ForceKill() {
	// Force kill the server
	if h.listener != nil {
		h.listener.Close()
		h.listener = nil
		h.log(models.LevelInfo, "Listener closed", nil)
	}
	h.srv.Close()
	h.log(models.LevelInfo, "Server force killed", nil)
}
