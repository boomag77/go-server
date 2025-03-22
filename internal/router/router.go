package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"telegram_server/internal/models"
)

type Logger interface {
	LogEvent(string)
}

type Database interface {
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]models.Message, error)
}

type HttpServer interface {
	SetHandler(string, http.HandlerFunc)
}

// Router is a service that routes incoming requests
type RouterImpl struct {
	logger   Logger
	database Database
}

type Router interface {
	PingHandler(w http.ResponseWriter, r *http.Request)
	MessageHandler(w http.ResponseWriter, r *http.Request)
}

// NewRouter creates a new Router
func NewRouter(l Logger, db Database) (Router, error) {
	return &RouterImpl{
		logger:   l,
		database: db,
	}, nil
}

// GET Handler /ping (server check)
func (rt *RouterImpl) PingHandler(w http.ResponseWriter, r *http.Request) {
	rt.logger.LogEvent("Ping request received")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "pong"}`)
}

// POST Handler /message (receive JSON-message)
func (rt *RouterImpl) MessageHandler(w http.ResponseWriter, r *http.Request) {
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
	rt.logger.LogEvent(logString)

	// Saving message to database
	if err := rt.database.SaveMessage(context.Background(), msg.Username, msg.Text); err != nil {
		rt.logger.LogEvent("Error while saving message to database: " + err.Error())
	} else {
		rt.logger.LogEvent("Message saved successfully")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})

	//TO DELETE
	fmt.Println(rt.database.GetMessages(context.Background()))
}
