package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	//"telegram_server/internal/awsclient"
	"telegram_server/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot is a service that interacts with Telegram bot

type BotImpl struct {
	logger   Logger
	database Database
}

type Bot interface {
	SendMessage(chatID int64, text string) error
	WebHookHandler(w http.ResponseWriter, r *http.Request)
}

type Logger interface {
	LogEvent(string)
}

type Database interface {
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]models.Message, error)
}

type AWSClient interface {
	GetBotToken(ctx context.Context) (string, error)
}

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

var botToken string

// NewBot creates a new Bot
func NewBot(l Logger, db Database) (Bot, error) {
	// newAws, err := awsclient.NewAWSClient(l)
	// if err != nil {
	// 	return nil, err
	// }
	// tkn, err := newAws.GetBotToken(context.Background())
	// if err != nil {
	// 	return nil, err
	// }
	botToken = "tkn"
	return &BotImpl{
		logger:   l,
		database: db,
	}, nil
}

func (b *BotImpl) SendMessage(chatID int64, text string) error {

	url := "https://api.telegram.org/bot" + botToken + "/sendMessage"
	data := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}
	body, err := json.Marshal(data)
	if err != nil {
		b.logger.LogEvent("Error while marshaling JSON: " + err.Error())
		return err
	}

	response, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		b.logger.LogEvent("Error while sending response message: " + err.Error())
		return err
	}
	defer response.Body.Close()

	b.logger.LogEvent("Message sent! " + response.Status + " " + fmt.Sprint(response.StatusCode))
	return nil
}

// webhook Handler
func (b *BotImpl) WebHookHandler(w http.ResponseWriter, r *http.Request) {

	var update tgbotapi.Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {

		logString := "Error while decoding webhook update: " + err.Error()
		b.logger.LogEvent(logString)
		http.Error(w, "Error while decoding", http.StatusBadRequest)
		return
	}
	if update.Message != nil {
		userName := update.Message.From.UserName
		messageText := update.Message.Text

		logString := "Received message from: " + userName + ", text: " + messageText
		b.logger.LogEvent(logString)

		// Saving message to database
		if err := b.database.SaveMessage(context.Background(), userName, messageText); err != nil {
			b.logger.LogEvent("Error while saving message to database: " + err.Error())
		} else {
			b.logger.LogEvent("Message saved successfully")
		}

		// TO DELETE
		fmt.Println(b.database.GetMessages(context.Background()))

		responseText := "Hi, " + userName + "! You wrote: " + messageText
		b.SendMessage(update.Message.Chat.ID, responseText)

	}

	w.WriteHeader(http.StatusOK)
}
