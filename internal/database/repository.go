package database

import (
	"context"
	"strconv"
	"telegram_server/internal/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SaveMessage(ctx context.Context, db *pgxpool.Pool, username, text string) error {
	_, err := db.Exec(ctx, "INSERT INTO messages (username, text) VALUES ($1, $2)", username, text)
	if err != nil {
		logger.LogEvent("Error while saving message: " + err.Error())
		return err
	}
	logger.LogEvent("Message saved successfully")
	return nil
}

func GetMessages(ctx context.Context, db *pgxpool.Pool) ([]Message, error) {
	rows, err := db.Query(ctx, "SELECT id, username, text FROM messages")
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
