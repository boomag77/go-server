package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type SendMessageRequest struct {
	ChatID int64 `json:"chat_id"`
	Text string  `json:"text"`
}

var (
	logFile *os.File
	logChan chan string
	wg	sync.WaitGroup
)


func getBotToken() string {
	
	awsRegion := "us-east-2"

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})

	if err != nil {

		logString := fmt.Sprintf("Error when trying to create AWS-session: %s", err)
		logEvent(logString)
		return ""
	}


	ssmsvc := ssm.New(sess)

	param, err := ssmsvc.GetParameter((&ssm.GetParameterInput{
		Name: 		aws.String("/kiddokey-bot/BOT_TOKEN"),
		WithDecryption: aws.Bool(true),
	}))
	
	if err != nil {

		logString := fmt.Sprintf("Error while getting token from AWS Parameter Store: %s", err)
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
		logString, ok := <- logChan
		if !ok {
			return
		}
		log.Println(logString)
	}
}

// GET Handler /ping (server check)
func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,`{"message": "pong"}`)
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
		Text string `json:"text"`
	}

	// Decode JSON-request to struct
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Logging user message to console
	log.Printf("Message from %s: %s\n", msg.Username, msg.Text)
	logString := fmt.Sprintf("Received message from: %s, text: %s", msg.Username, msg.Text)
	logEvent(logString)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}


// sendin message to BOT_TOKEN
func sendMessage(chatID int64, text string) {

	botToken := getBotToken()
	if botToken == "" {

		logString := fmt.Sprintf("Error: Empty token!")
		logEvent(logString)
		return
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	data := SendMessageRequest{
		ChatID: chatID,
		Text: text,
	}

	body, _ := json.Marshal(data)
	response, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {

		logString := fmt.Sprintf("Error while sending response message: %s", err)
		logEvent(logString)
		return
	}
	defer response.Body.Close()

	logString := fmt.Sprintf("Message sent!")
	logEvent(logString)

}

// webhook Handler
func webHookHandler(w http.ResponseWriter, r *http.Request) {
	
	var update tgbotapi.Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {

		logString := fmt.Sprintf("Error while decoding webhook update: %s", err)
		logEvent(logString)
		http.Error(w, "Error while decoding", http.StatusBadRequest)
		return
	}
	if update.Message != nil {
		userName := update.Message.From.UserName
		messageText := update.Message.Text

		logString := fmt.Sprintf("Received message from: %s, text: %s", userName, messageText)
		logEvent(logString)

		responseText := fmt.Sprintf("Hi, %s! You wrote: \"%s\"", userName, messageText)
		sendMessage(update.Message.Chat.ID, responseText)

	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	initLogger()

	defer logFile.Close()
	defer wg.Wait()
	defer close(logChan)

	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/message", messageHandler)
	http.HandleFunc("/webhook", webHookHandler)

	log.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
