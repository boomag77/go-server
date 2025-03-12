package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
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

var (
	logFile *os.File
	logChan chan string
	wg      sync.WaitGroup
)

func getBotToken() string {

	awsRegion := "us-east-2"

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})

	if err != nil {

		logString := "Error when trying to create AWS-session: " + err.Error()
		logEvent(logString)
		return ""
	}

	ssmsvc := ssm.New(sess)

	param, err := ssmsvc.GetParameter((&ssm.GetParameterInput{
		Name:           aws.String("/kiddokey-bot/BOT_TOKEN"),
		WithDecryption: aws.Bool(true),
	}))

	if err != nil {

		logString := "Error while getting token from AWS Parameter Store: " + err.Error()
		logEvent(logString)
		return ""
	}

	return *param.Parameter.Value
}

func initLogger() {

	const logFileName string = "server.log"
	var err error

	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(logFile)

	logChan = make(chan string, 100)

	numWorkers := runtime.NumCPU()
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go logWorker()
	}
}

func logWorker() {
	defer wg.Done()
	for {
		logString, ok := <-logChan
		if !ok {
			return
		}
		log.Println(logString)
	}
}

// GET Handler /ping (server check)
func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `{"message": "pong"}`)
}

func logEvent(logString string) {
	select {
	case logChan <- logString:

	default:
		log.Println("WARNING: log channel is full, dropping log!")
	}
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
	logEvent(logString)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

// sendin message to BOT_TOKEN
func sendMessage(chatID int64, text string) {

	botToken := getBotToken()
	if botToken == "" {
		logString := "Error: Empty token!"
		logEvent(logString)
		return
	}

	url := "https://api.telegram.org/bot" + botToken + "/sendMessage"

	data := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	// Marshal the data to JSON
	body, err := json.Marshal(data)
	if err != nil {
		logString := "Error while marshaling JSON: " + err.Error()
		logEvent(logString)
		return
	}

	// Send the POST request to the Telegram API
	response, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logString := "Error while sending response message:" + err.Error()
		logEvent(logString)
		return
	}
	defer response.Body.Close()

	logString := "Message sent!"
	logEvent(logString)
}

// webhook Handler
func webHookHandler(w http.ResponseWriter, r *http.Request) {

	var update tgbotapi.Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {

		logString := "Error while decoding webhook update: " + err.Error()
		logEvent(logString)
		http.Error(w, "Error while decoding", http.StatusBadRequest)
		return
	}
	if update.Message != nil {
		userName := update.Message.From.UserName
		messageText := update.Message.Text

		logString := "Received message from: " + userName + ", text: " + messageText
		logEvent(logString)

		responseText := "Hi, " + userName + "! You wrote: " + messageText
		sendMessage(update.Message.Chat.ID, responseText)

	}

	w.WriteHeader(http.StatusOK)
}

// shutdown server
func shutdownServer(server *http.Server) {

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	signal := <-sigChan
	logString := "Received signal: " + signal.String()
	logEvent(logString)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	if err := server.Shutdown(ctx); err != nil {

		logString := "Error while shutting down server: " + err.Error()
		logEvent(logString)
	}

	logString = "Server is down!"
	logEvent(logString)

	os.Exit(0)
}

// start server
func startServer() *http.Server {

	server := &http.Server{
		Addr: ":8080",
	}

	go func() {

		logString := "Starting server on port 8080..."
		logEvent(logString)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {

			logString := "Error while starting server: " + err.Error()
			logEvent(logString)
		}
	}()

	return server
}

func main() {
	initLogger()

	defer logFile.Close()
	defer wg.Wait()
	defer close(logChan)

	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/message", messageHandler)
	http.HandleFunc("/webhook", webHookHandler)

	server := startServer()
	shutdownServer(server)
}
