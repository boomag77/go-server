package database

import (
	"context"
	"fmt"
	"strconv"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"
)

func (db DatabaseImpl) SaveMessage(ctx context.Context, username, text string) error {
	_, err := db.pool.Exec(ctx, "INSERT INTO messages (username, text) VALUES ($1, $2)", username, text)
	if err != nil {
		return err
	}
	return nil
}

func (db DatabaseImpl) GetMessages(ctx context.Context) ([]models.Message, error) {
	rows, err := db.pool.Query(ctx, "SELECT id, username, text FROM messages")
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message

	for rows.Next() {
		var message models.Message
		if err := rows.Scan(&message.ID, &message.UserName, &message.Text); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan messages: %w", err)
	}

	db.logger.LogEvent(LogMessage{
		Level:   models.LevelInfo,
		Service: contracts.DatabaseName,
		Message: "Messages retrieved successfully - " + strconv.Itoa(len(messages)) + " messages found",
		Err:     nil,
	})
	return messages, nil
}
