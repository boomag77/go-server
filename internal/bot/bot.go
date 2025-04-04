package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	//"telegram_server/internal/awsclient"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot is a service that interacts with Telegram bot
type Bot = contracts.Bot
type Logger = contracts.Logger
type Database = contracts.Database
type LogMessage = contracts.LogMessage
type Message = models.Message

type BotImpl struct {
	logger   Logger
	database Database
}

type AWSClient interface {
	GetBotToken(ctx context.Context) (string, error)
}

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

var url string

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
	botToken := "tkn"
	url = "https://api.telegram.org/bot" + botToken + "/sendMessage"
	return &BotImpl{
		logger:   l,
		database: db,
	}, nil
}

func (b *BotImpl) SendMessage(chatID int64, text string) error {

	data := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error while marshaling JSON: %w", err)
	}

	response, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("error while sending response message: %w", err)
	}
	defer response.Body.Close()

	b.logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.BotName,
		Message: "Message sent! " + response.Status + " " + fmt.Sprint(response.StatusCode),
		Err:     nil,
	})
	return nil
}

// webhook Handler
func (b *BotImpl) WebHookHandler(w http.ResponseWriter, r *http.Request) {

	var update tgbotapi.Update

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {

		b.logger.LogEvent(LogMessage{
			Level:   models.LevelError,
			Service: contracts.BotName,
			Message: "Error while decoding webhook update",
			Err:     err,
		})
		http.Error(w, "Error while decoding", http.StatusBadRequest)
		return
	}
	if update.Message != nil {
		userName := update.Message.From.UserName
		messageText := update.Message.Text

		var builder strings.Builder
		builder.WriteString("Received message from: ")
		builder.WriteString(userName)
		builder.WriteString(", text: ")
		builder.WriteString(messageText)

		b.logger.LogEvent(LogMessage{
			Level:   models.LevelInfo,
			Service: contracts.BotName,
			Message: builder.String(),
			Err:     nil,
		})

		// Saving message to database
		if err := b.database.SaveMessage(context.Background(), userName, messageText); err != nil {
			b.logger.LogEvent(LogMessage{
				Level:   models.LevelError,
				Service: contracts.DatabaseName,
				Message: "Error while saving received message to database",
				Err:     err,
			})
		} else {
			b.logger.LogEvent(LogMessage{
				Level:   models.LevelInfo,
				Service: contracts.DatabaseName,
				Message: "Received message has been saved successfully",
				Err:     nil,
			})
		}

		// TO DELETE
		fmt.Println(b.database.GetMessages(context.Background()))

		responseText := "Hi, " + userName + "! You wrote: " + messageText
		if err := b.SendMessage(update.Message.Chat.ID, responseText); err != nil {
			b.logger.LogEvent(LogMessage{
				Level:   models.LevelError,
				Service: contracts.BotName,
				Message: "Error while sending message to user " + userName,
				Err:     err,
			})
		} else {
			b.logger.LogEvent(LogMessage{
				Level:   models.LevelInfo,
				Service: contracts.BotName,
				Message: "Message to user " + userName + " sent successfully",
				Err:     nil,
			})
		}

	}

	w.WriteHeader(http.StatusOK)
}
