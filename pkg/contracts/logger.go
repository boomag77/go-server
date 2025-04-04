package contracts

import (
	"context"
	"telegram_server/pkg/models"
)

type LogMessage = models.LogMessage

type Logger interface {
	Service
	LogEvent(ctx context.Context, msg LogMessage)
}
