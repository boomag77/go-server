package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"telegram_server/config"
	"telegram_server/internal/logger"
	"time"
)

var server *http.Server

// GetServerForTesting returns the server for testing
// for testing purposes
func GetServerForTesting() *http.Server {
	return server
}

// SetServerForTesting sets the server for testing
// for testing purposes
func SetServerForTesting(s *http.Server) {
	server = s
}

// start server
func Start() error {
	server = &http.Server{
		Addr: config.ServerPort,
	}

	errChan := make(chan error, 1)

	go func() {
		logger.LogEvent("Starting server on port " + config.ServerPort)
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	select {
	case err := <-errChan:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// shutdown server
func Shutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	logger.LogEvent("Received signal: " + sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.LogEvent("Error while shutting down server: " + err.Error())
	}

	logger.LogEvent("Server is down!")
}
