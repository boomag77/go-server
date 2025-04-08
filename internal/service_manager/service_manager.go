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
type LogLevel = models.LogLevel
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

func (sm *ServiceManagerImpl) log(level LogLevel, message string, err error) {
	sm.logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   level,
			Service: contracts.ServiceManagerName,
			Message: message,
			Err:     err,
		})
}

func (sm *ServiceManagerImpl) snapshotServices() map[string]Service {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	snapshot := make(map[string]Service, len(sm.services))
	for name, service := range sm.services {
		snapshot[name] = service
	}
	return snapshot
}

func (sm *ServiceManagerImpl) setServiceStatus(name string, status ServiceStatus) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.statuses[name] = status
}

func (sm *ServiceManagerImpl) isReadyToStart(name string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	_, registered := sm.services[name]
	if !registered {
		return false
	}
	if sm.statuses[name] == models.ServiceStatusRunning {
		return false
	}
	if sm.statuses[name] == models.ServiceStatusStarting {
		return false
	}
	if sm.statuses[name] == models.ServiceStatusStopping {
		return false
	}
	return true
}

func (sm *ServiceManagerImpl) StartService(ctx context.Context, name string) error {

	if !sm.isReadyToStart(name) {
		return fmt.Errorf("service %s not registered or already starting/stopping", name)
	}
	service := sm.services[name]
	sm.setServiceStatus(name, models.ServiceStatusStarting)

	err := service.Start(ctx)
	if err != nil {
		sm.setServiceStatus(name, models.ServiceStatusStopped)
		return err
	}

	sm.setServiceStatus(name, models.ServiceStatusRunning)
	sm.log(models.LevelInfo,
		"Service "+name+" started successfully",
		nil)

	return nil

}

func (sm *ServiceManagerImpl) StartAllServices(ctx context.Context) {
	var notStarted []string
	var notStartedMutex sync.Mutex
	var wg sync.WaitGroup

	services := sm.snapshotServices()

	for name := range services {

		name := name
		//service := service

		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := sm.StartService(ctxWithTimeout, name); err != nil {

				notStartedMutex.Lock()
				notStarted = append(notStarted, name)
				notStartedMutex.Unlock()

				sm.log(models.LevelError,
					"Service "+name+" failed to start: "+err.Error(),
					err)

				return
			}
			sm.log(models.LevelInfo,
				"Service "+name+" started successfully",
				nil)
		}(name)

	}
	wg.Wait()
	if len(notStarted) > 0 {
		sm.log(
			models.LevelError,
			"Services failed to start: "+fmt.Sprintf("%v", notStarted),
			nil)
	} else {
		sm.log(
			models.LevelInfo,
			"All services started successfully",
			nil)
	}
}

func (sm *ServiceManagerImpl) StopService(ctx context.Context, name string) error {

	var service Service
	var err error

	switch sm.GetServiceStatus(name) {
	case models.ServiceStatusRunning:
		sm.mu.RLock()
		service = sm.services[name]
		sm.mu.RUnlock()
	case models.ServiceStatusStopped:
		return fmt.Errorf("service %s already stopped", name)
	case models.ServiceStatusStarting:
		return fmt.Errorf("service %s is starting", name)
	case models.ServiceStatusStopping:
		return fmt.Errorf("service %s is already stopping", name)
	default:
		return fmt.Errorf("service %s not registered", name)
	}

	sm.setServiceStatus(name, models.ServiceStatusStopping)
	for i := 0; i < 3; i++ {
		err = service.Shutdown(ctx)
		if err == nil {
			sm.setServiceStatus(name, models.ServiceStatusStopped)
			return nil
		}
		sm.log(models.LevelWarning,
			"Shutdown attempt "+fmt.Sprintf("%d", i+1)+" for service "+name+" failed: ",
			err,
		)

	}
	sm.setServiceStatus(name, models.ServiceStatusRunning)
	return fmt.Errorf("failed to stop service %s: %w", name, err)

}
func (sm *ServiceManagerImpl) StopAllServices(ctx context.Context) error {
	var notStopped []string
	var notStoppedMutex sync.Mutex
	var wg sync.WaitGroup
	services := sm.snapshotServices()

	for name := range services {
		sm.setServiceStatus(name, models.ServiceStatusStopping)
		name := name
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			err := sm.StopService(ctxWithTimeout, name)

			if err != nil {
				notStoppedMutex.Lock()
				notStopped = append(notStopped, name)
				notStoppedMutex.Unlock()

				sm.log(models.LevelError,
					"Service "+name+" failed to stop: ",
					err)

				return
			}
			sm.log(models.LevelInfo,
				"Service "+name+" stopped successfully",
				nil)
		}(name)

	}
	wg.Wait()
	if len(notStopped) > 0 {
		sm.log(
			models.LevelError,
			"Services failed to stop: "+fmt.Sprintf("%v", notStopped),
			nil)
	} else {
		sm.log(
			models.LevelInfo,
			"All services stopped successfully",
			nil)
	}
	return nil
}

func (sm *ServiceManagerImpl) GetServiceStatus(name string) ServiceStatus {

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	switch sm.statuses[name] {
	case models.ServiceStatusRunning:
		return models.ServiceStatusRunning
	case models.ServiceStatusStopped:
		return models.ServiceStatusStopped
	case models.ServiceStatusStarting:
		return models.ServiceStatusStarting
	case models.ServiceStatusStopping:
		return models.ServiceStatusStopping
	default:
		return models.ServiceStatusNotRegistered
	}
}
