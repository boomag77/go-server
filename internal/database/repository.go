package database

import (
	"context"
	"strconv"
	"telegram_server/internal/models"
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
		db.logger.LogEvent("Error while getting messages: " + err.Error())
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message

	for rows.Next() {
		var message models.Message
		if err := rows.Scan(&message.ID, &message.UserName, &message.Text); err != nil {
			db.logger.LogEvent("Error while scanning message: " + err.Error())
			return nil, err
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		db.logger.LogEvent("Error after scanning rows: " + err.Error())
		return nil, err
	}
	db.logger.LogEvent("Retrieved " + strconv.Itoa(len(messages)) + " messages ")
	return messages, nil
}
