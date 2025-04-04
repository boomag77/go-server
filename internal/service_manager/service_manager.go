package service_manager

import (
	"context"
	"fmt"
	"sync"
	"telegram_server/pkg/contracts"
	"telegram_server/pkg/models"
	"time"
)

type Logger = contracts.Logger
type Service = contracts.Service
type LogMessage = models.LogMessage
type ServiceManager = contracts.ServiceManager
type ServiceStatus = models.ServiceStatus

type ServiceManagerImpl struct {
	services map[string]Service
	statuses map[string]models.ServiceStatus
	logger   Logger
	mu       sync.RWMutex
}

func NewServiceManager(logger contracts.Logger) ServiceManager {
	return &ServiceManagerImpl{
		services: make(map[string]contracts.Service),
		statuses: make(map[string]models.ServiceStatus),
		logger:   logger,
	}
}

func (sm *ServiceManagerImpl) RegisterService(name string, service contracts.Service) error {
	sm.mu.Lock()

	if _, exists := sm.services[name]; exists {
		sm.mu.Unlock()
		return fmt.Errorf("service %s already registered", name)
	}

	sm.services[name] = service
	sm.statuses[name] = models.ServiceStatusStopped
	sm.mu.Unlock()

	sm.logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   models.LevelInfo,
			Service: "ServiceManager",
			Message: "Service " + name + " registered",
			Err:     nil,
		})
	return nil
}

func (sm *ServiceManagerImpl) UnregisterService(name string) error {
	sm.mu.Lock()

	service, registered := sm.services[name]
	if !registered {
		sm.mu.Unlock()
		return fmt.Errorf("service %s not registered", name)
	}
	if sm.statuses[name] == models.ServiceStatusRunning {
		err := service.Shutdown(context.Background())
		if err != nil {
			sm.mu.Unlock()
			return fmt.Errorf("failed to shutdown service %s: %w", name, err)
		}
		sm.logger.LogEvent(
			context.Background(),
			LogMessage{
				Level:   models.LevelInfo,
				Service: "ServiceManager",
				Message: "Service " + name + " stopped before unregistering",
				Err:     nil,
			})
	}
	delete(sm.services, name)
	sm.mu.Unlock()

	sm.logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   models.LevelInfo,
			Service: "ServiceManager",
			Message: "Service " + name + " unregistered",
			Err:     nil,
		})
	return nil
}

func (sm *ServiceManagerImpl) StartAllServices(ctx context.Context) {
	var notStarted []string
	var notStartedMutex sync.Mutex
	var wg sync.WaitGroup
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for name, service := range sm.services {

		name := name
		service := service

		wg.Add(1)

		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			err := service.Start(ctx)
			if err != nil {
				sm.mu.Lock()
				sm.statuses[name] = models.ServiceStatusStopped
				sm.mu.Unlock()

				notStartedMutex.Lock()
				notStarted = append(notStarted, name)
				notStartedMutex.Unlock()

				sm.logger.LogEvent(
					context.Background(),
					LogMessage{
						Level:   models.LevelError,
						Service: "ServiceManager",
						Message: "Service " + name + " failed to start",
						Err:     err,
					})
				return
			}
			sm.mu.Lock()
			sm.statuses[name] = models.ServiceStatusRunning
			sm.mu.Unlock()

			sm.logger.LogEvent(
				context.Background(),
				LogMessage{
					Level:   models.LevelInfo,
					Service: "ServiceManager",
					Message: "Service " + name + " started successfully",
					Err:     nil,
				})
		}()

	}
	wg.Wait()
	if len(notStarted) > 0 {
		sm.logger.LogEvent(
			context.Background(),
			LogMessage{
				Level:   models.LevelError,
				Service: "ServiceManager",
				Message: "Some services failed to start: " + fmt.Sprintf("%v", notStarted),
				Err:     nil,
			})
	} else {
		sm.logger.LogEvent(
			context.Background(),
			LogMessage{
				Level:   models.LevelInfo,
				Service: "ServiceManager",
				Message: "All services started successfully",
				Err:     nil,
			})
	}
}

func (sm *ServiceManagerImpl) StartService(ctx context.Context, name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	service, registered := sm.services[name]
	if !registered {
		return fmt.Errorf("service %s not found", name)
	}
	if sm.statuses[name] == models.ServiceStatusRunning {
		return fmt.Errorf("service %s is already running", name)
	}
	if sm.statuses[name] == models.ServiceStatusStarting {
		return fmt.Errorf("service %s is already starting", name)
	}
	if sm.statuses[name] == models.ServiceStatusStopping {
		return fmt.Errorf("service %s is stopping, please wait", name)
	}

	err := service.Start(ctx)
	sm.statuses[name] = models.ServiceStatusStarting
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w", name, err)
	}
	sm.statuses[name] = models.ServiceStatusRunning

	sm.logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   models.LevelInfo,
			Service: "ServiceManager",
			Message: "Service " + name + " started successfully",
			Err:     nil,
		})
	return nil

}

func (sm *ServiceManagerImpl) ShutdownService(ctx context.Context, name string) error {

	sm.mu.Lock()

	service, registered := sm.services[name]
	if !registered {
		return fmt.Errorf("service %s not registered", name)
	}
	sm.mu.Unlock()

	sm.mu.Lock()
	sm.statuses[name] = models.ServiceStatusStopping
	err := service.Shutdown(ctx)
	if err != nil {
		sm.mu.Unlock()
		return fmt.Errorf("failed to shutdown service %s: %w", name, err)
	}
	sm.statuses[name] = models.ServiceStatusStopped
	sm.mu.Unlock()

	sm.logger.LogEvent(
		context.Background(),
		contracts.LogMessage{
			Level:   models.LevelInfo,
			Service: "ServiceManager",
			Message: "Service " + name + " stopped successfully",
			Err:     nil,
		})
	return nil
}
func (sm *ServiceManagerImpl) StopAll(ctx context.Context) error {
	for name, service := range sm.services {
		err := service.Shutdown(ctx)
		if err != nil {
			return fmt.Errorf("failed to stop service %s: %w", name, err)
		}
		sm.logger.LogEvent(
			context.Background(),
			contracts.LogMessage{
				Level:   models.LevelInfo,
				Service: "ServiceManager",
				Message: "Service " + name + " stopped successfully",
				Err:     nil,
			})
	}
	return nil
}
func (sm *ServiceManagerImpl) StartAll(ctx context.Context) error {
	for name, service := range sm.services {
		err := service.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to start service %s: %w", name, err)
		}
		sm.logger.LogEvent(
			context.Background(),
			contracts.LogMessage{
				Level:   models.LevelInfo,
				Service: "ServiceManager",
				Message: "Service " + name + " started successfully",
				Err:     nil,
			})
	}
	return nil
}

func (sm *ServiceManagerImpl) GetServiceCount(ctx context.Context) int {
	return len(sm.services)
}

func (sm *ServiceManagerImpl) GetServiceStatus(name string) (ServiceStatus, error) {
	_, registered := sm.services[name]
	if !registered {
		return nil, fmt.Errorf("service %s not found", name)
	}
	status := sm.statuses[name]
	return status, nil
}
func (sm *ServiceManagerImpl) ShutdownAllServices(ctx context.Context) error {
	for _, service := range sm.services {
		err := service.Shutdown(ctx)
		if err != nil {
			return fmt.Errorf("failed to shutdown service: %w", err)
		}
	}
	return nil
}
