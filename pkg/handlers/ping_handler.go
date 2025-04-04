package handlers

import (
	"context"
	"fmt"
	"net/http"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"
)

// GET Handler /ping (server check)
func (ph PostHandler) PingHandler(w http.ResponseWriter, r *http.Request) {
	ph.logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   models.LevelInfo,
			Service: contracts.HandlerName,
			Message: "Ping request received",
			Err:     nil,
		})
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "pong"}`)
}
