package database

import (
	"context"
	"strconv"
	"telegram_server/internal/logger"

)

func SaveMessage(ctx context.Context, username, text string) error {
	_, err := Pool.Exec(ctx, "INSERT INTO messages (username, text) VALUES ($1, $2)", username, text)
	if err != nil {
		return err
	}
	return nil
}

func GetMessages(ctx context.Context) ([]Message, error) {
	rows, err := Pool.Query(ctx, "SELECT id, username, text FROM messages")
	if err != nil {
		logger.LogEvent("Error while getting messages: " + err.Error())
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.UserName, &message.Text); err != nil {
			logger.LogEvent("Error while scanning message: " + err.Error())
			return nil, err
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		logger.LogEvent("Error after scanning rows: " + err.Error())
		return nil, err
	}
	logger.LogEvent("Retrieved " + strconv.Itoa(len(messages)) + " messages ")
	return messages, nil
}
