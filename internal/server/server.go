package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"telegram_server/config"
	"telegram_server/internal/database"
	"telegram_server/internal/logger"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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

// GET Handler /ping (server check)
func PingHandler(w http.ResponseWriter, r *http.Request) {
	logger.LogEvent("Ping request received")
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "pong"}`)
}

// POST Handler /message (receive JSON-message)
func MessageHandler(w http.ResponseWriter, r *http.Request) {
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
	logger.LogEvent(logString)

	// Saving message to database

	if err := database.SaveMessage(context.Background(), msg.Username, msg.Text); err != nil {
		logger.LogEvent("Error while saving message to database: " + err.Error())
	} else {
		logger.LogEvent("Message saved successfully")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})

	//TO DELETE
	fmt.Println(database.GetMessages(context.Background()))
}

// sending message to user
func sendMessage(chatID int64, text string) {
	botToken := getBotToken()
	if botToken == "" {
		logger.LogEvent("Error: Empty token!")
		return
	}

	url := "https://api.telegram.org/bot" + botToken + "/sendMessage"
	data := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	body, err := json.Marshal(data)
	if err != nil {
		logger.LogEvent("Error while marshaling JSON: " + err.Error())
		return
	}

	response, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.LogEvent("Error while sending response message: " + err.Error())
		return
	}
	defer response.Body.Close()

	logger.LogEvent("Message sent! " + response.Status + " " + fmt.Sprint(response.StatusCode))
}

// webhook Handler
func WebHookHandler(w http.ResponseWriter, r *http.Request) {

	var update tgbotapi.Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {

		logString := "Error while decoding webhook update: " + err.Error()
		logger.LogEvent(logString)
		http.Error(w, "Error while decoding", http.StatusBadRequest)
		return
	}
	if update.Message != nil {
		userName := update.Message.From.UserName
		messageText := update.Message.Text

		logString := "Received message from: " + userName + ", text: " + messageText
		logger.LogEvent(logString)

		// Saving message to database
		if err := database.SaveMessage(context.Background(), userName, messageText); err != nil {
			logger.LogEvent("Error while saving message to database: " + err.Error())
		} else {
			logger.LogEvent("Message saved successfully")
		}

		// TO DELETE
		fmt.Println(database.GetMessages(context.Background()))

		responseText := "Hi, " + userName + "! You wrote: " + messageText
		sendMessage(update.Message.Chat.ID, responseText)

	}

	w.WriteHeader(http.StatusOK)
}

// Add global variable for session creation to allow injection in tests
var newSession = session.NewSession

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func getBotToken() string {
	awsRegion := "us-east-2"

	sess, err := newSession(&aws.Config{
		Region: aws.String(awsRegion),
	})
	if err != nil {
		logger.LogEvent("Error when trying to create AWS-session: " + err.Error())
		return ""
	}

	ssmsvc := ssm.New(sess)
	param, err := ssmsvc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String("/kiddokey-bot/BOT_TOKEN"),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {

		logger.LogEvent("Error while getting token from AWS Parameter Store: " + err.Error())
		return ""
	}

	return *param.Parameter.Value
}
