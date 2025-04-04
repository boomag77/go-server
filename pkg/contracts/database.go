package contracts

import (
	"context"
	"telegram_server/pkg/models"
)

type Message = models.Message

type Database interface {
	Connect(ctx context.Context) error
	SaveMessage(ctx context.Context, username, text string) error
	GetMessages(ctx context.Context) ([]Message, error)
	Ping() error
	Disconnect()
}
