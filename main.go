package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

var (
	logFile *os.File
	logMutex sync.Mutex
)

func initLogger() {
	
	const logFileName string = "server.log"
	var err error

	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(logFile)
}

// GET Handler /ping (server check)
func pingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w,`{"message": "pong"}`)
}

func logEvent(logString string) {

	logMutex.Lock()
	defer logMutex.Unlock()
	

	log.Println(logString)
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

func main() {
	initLogger()
	defer logFile.Close()
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/message", messageHandler)

	log.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
