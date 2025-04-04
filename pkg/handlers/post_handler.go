package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"
)

type Logger = contracts.Logger
type LogMessage = contracts.LogMessage
type Database = contracts.Database

type PostHandler struct {
	logger Logger
	db     Database
}

// POST Handler /message (receive JSON-message)
func (ph PostHandler) MessageHandler(w http.ResponseWriter, r *http.Request) {
	var msg struct {
		Username string `json:"username"`
		Text     string `json:"text"`
	}

	// Decode JSON-request to struct
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Logging user message to console
	logString := "Received message from: " + msg.Username + ", text: " + msg.Text
	ph.logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   models.LevelInfo,
			Service: contracts.HandlerName,
			Message: logString,
			Err:     nil,
		},
	)

	// Saving message to database
	if err := ph.db.SaveMessage(context.Background(), msg.Username, msg.Text); err != nil {
		ph.logger.LogEvent(
			context.Background(),
			LogMessage{
				Level:   models.LevelError,
				Service: contracts.DatabaseName,
				Message: "Error while saving message to database",
				Err:     err,
			},
		)
	} else {
		ph.logger.LogEvent(
			context.Background(),
			LogMessage{
				Level:   models.LevelInfo,
				Service: contracts.DatabaseName,
				Message: "Message saved successfully",
				Err:     nil,
			},
		)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})

	//TO DELETE
	fmt.Println(ph.db.GetMessages(context.Background()))
}
