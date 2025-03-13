package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SaveMessage(ctx context.Context, db *pgxpool.Pool, username, text string) error {
	_, err := db.Exec(ctx, "INSERT INTO messages (username, text) VALUES ($1, $2)", username, text)
	if err != nil {
		log.Printf("Error while saving message: %v", err)
		return err
	}
	log.Println("Message saved successfully")
	return nil
}

func GetMessages(ctx context.Context, db *pgxpool.Pool) ([]Message, error) {
	rows, err := db.Query(ctx, "SELECT id, username, text FROM messages")
	if err != nil {
		log.Printf("Error while getting messages: %v", err)
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.UserName, &message.Text); err != nil {
			log.Printf("Error while scanning message: %v", err)
			return nil, err
		}
		messages = append(messages, message)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Error after scanning rows: %v", err)
		return nil, err
	}

	log.Printf("Retrieved %d messages", len(messages))
	return messages, nil
}
