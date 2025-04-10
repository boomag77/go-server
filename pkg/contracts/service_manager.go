package contracts

import (
	"context"
	"telegram_server/pkg/models"
)

type ServiceStatus = models.ServiceStatus

type ServiceManager interface {
	RegisterService(name string, service Service) error
	UnregisterService(name string) error
	StartAllServices(ctx context.Context)
	StopAllServices(ctx context.Context) error
	StartService(ctx context.Context, name string) error
	StopService(ctx context.Context, name string) error
	GetServiceStatus(name string) ServiceStatus // RLock is used here
	RestartService(ctx context.Context, name string) error
}

// ServiceManager is an interface that defines methods for managing services.
// It allows for registering, unregistering, starting, and shutting down services.
// It also provides methods to get the status and count of services.
