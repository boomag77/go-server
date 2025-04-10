package contracts

import "context"

type ServiceName = string

const (
	BotName            string = "BOT"
	DatabaseName       string = "DATABASE"
	LoggerName         string = "LOGGER"
	HTTPServerName     string = "HTTPSERVER"
	HandlerName        string = "HANDLER"
	ServiceManagerName string = "SERVICEMANAGER"
)

type Service interface {
	Start(ctx context.Context) error
	IsHealthy() bool
	Shutdown(ctx context.Context) error
	ForceKill()
}
