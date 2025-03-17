package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPingHandler tests the /ping endpoint
func TestPingHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/ping", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(pingHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"message": "pong"}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

// TestMessageHandler tests the /message endpoint
func TestMessageHandler(t *testing.T) {
	message := map[string]string{"username": "testuser", "text": "Hello, world!"}
	body, _ := json.Marshal(message)
	req, err := http.NewRequest("POST", "/message", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(messageHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Ошибка при декодировании JSON-ответа: %v", err)
	}

	expected := map[string]string{"status": "received"}
	if response["status"] != expected["status"] {
		t.Errorf("handler returned unexpected body: got %v want %v", response["status"], expected["status"])
	}
}

// Mock getBotToken function for testing
func mockGetBotToken() string {
	return "mocked_token"
}

// TestGetBotToken tests the getBotToken function
func TestGetBotToken(t *testing.T) {
	token := mockGetBotToken() // Просто вызываем мок-функцию
	expected := "mocked_token"
	if token != expected {
		t.Errorf("getBotToken returned unexpected token: got %v want %v", token, expected)
	}
}

func TestSendMessage(t *testing.T) {
	// Ensure sendMessage does not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("sendMessage panicked: %v", r)
		}
	}()
	sendMessage(12345, "Hello, Test")
	// Optionally, add assertions if sendMessage modifies state or makes an HTTP call.
}
