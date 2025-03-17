package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	//"log"
	"net/http"
	"os"
	"os/signal"

	// "runtime"
	// "sync"
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

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func getBotToken() string {
	awsRegion := "us-east-2"

	sess, err := session.NewSession(&aws.Config{
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

// GET Handler /ping (server check)
func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `{"message": "pong"}`)
}

// POST Handler /message (receive JSON-message)
func messageHandler(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

// sendin message to BOT_TOKEN
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
func webHookHandler(w http.ResponseWriter, r *http.Request) {

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

		responseText := "Hi, " + userName + "! You wrote: " + messageText
		sendMessage(update.Message.Chat.ID, responseText)

	}

	w.WriteHeader(http.StatusOK)
}

// shutdown server
func shutdownServer(server *http.Server) {
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

// start server
func startServer() *http.Server {

	server := &http.Server{
		Addr: ":8080",
	}

	go func() {

		logString := "Starting server on port 8080..."
		logger.LogEvent(logString)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {

			logString := "Error while starting server: " + err.Error()
			logger.LogEvent(logString)
		}
	}()

	return server
}

func main() {
	config.Init()

	logger.Init()
	defer logger.Close()

	dbPool, err := database.InitDB()
	if err != nil {
		logger.LogEvent("Error while initializing database: " + err.Error())
		return
	}
	defer database.CloseDB(dbPool)

	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/message", messageHandler)
	http.HandleFunc("/webhook", webHookHandler)

	server := startServer()
	shutdownServer(server)
}
