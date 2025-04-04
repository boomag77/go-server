package contracts

import "context"

type ServiceManager interface {
	RegisterService(name string, service Service) error
	UnregisterService(name string) error
	StartAllServices(ctx context.Context)
	ShutdownAllServices(ctx context.Context) error
	StartService(ctx context.Context, name string) error
	ShutdownService(ctx context.Context, name string) error
	GetServiceStatus(name string) (string, error)
	GetServiceCount(ctx context.Context) int
}

// ServiceManager is an interface that defines methods for managing services.
// It allows for registering, unregistering, starting, and shutting down services.
// It also provides methods to get the status and count of services.
