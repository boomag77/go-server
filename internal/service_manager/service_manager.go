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

type Config struct {
	Logger                   Logger
	StartServiceAttemptCount int
	StopServiceAttemptCount  int
	StartServiceTimeout      time.Duration
	StopServiceTimeout       time.Duration
}

type ServiceManagerImpl struct {
	config   Config
	services map[string]Service
	statuses map[string]models.ServiceStatus
	mu       sync.RWMutex
}

func (sm *ServiceManagerImpl) Watch(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			sm.log(models.LevelInfo,
				"Service manager watch stopped",
				nil)
			return

		case <-ticker.C:
			services := sm.snapshotServices()
			for name, service := range services {
				if service.IsHealthy() {
					sm.log(models.LevelInfo,
						"Service "+name+" is healthy",
						nil)
				} else {
					sm.log(models.LevelWarning,
						"Service "+name+" is unhealthy",
						nil)
					if err := sm.RestartService(ctx, name); err != nil {
						sm.log(models.LevelError,
							"Failed to restart service "+name,
							err)
					}
				}
			}
		}
	}
}

func NewServiceManager(cfg Config) ServiceManager {
	var defaultConfig = defaultConfig()

	if cfg.Logger == nil {
		panic("logger is required for ServiceManager")
	}
	if cfg.StartServiceAttemptCount <= 0 {
		cfg.StartServiceAttemptCount = defaultConfig.StartServiceAttemptCount
	}
	if cfg.StopServiceAttemptCount <= 0 {
		cfg.StopServiceAttemptCount = defaultConfig.StopServiceAttemptCount
	}
	if cfg.StartServiceTimeout <= 0 {
		cfg.StartServiceTimeout = defaultConfig.StartServiceTimeout
	}
	if cfg.StopServiceTimeout <= 0 {
		cfg.StopServiceTimeout = defaultConfig.StopServiceTimeout
	}
	return &ServiceManagerImpl{
		config:   cfg,
		services: make(map[string]contracts.Service),
		statuses: make(map[string]models.ServiceStatus),
	}
}

func defaultConfig() Config {
	return Config{
		StartServiceAttemptCount: 3,
		StopServiceAttemptCount:  3,
		StartServiceTimeout:      5 * time.Second,
		StopServiceTimeout:       5 * time.Second,
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

	sm.log(models.LevelInfo,
		"Service "+name+" registered successfully",
		nil)
	return nil
}

func (sm *ServiceManagerImpl) UnregisterService(name string) error {

	serviceStatus := sm.GetServiceStatus(name)

	if serviceStatus == models.ServiceStatusNotRegistered {
		return fmt.Errorf("service %s not registered", name)
	}

	if serviceStatus == models.ServiceStatusRunning {
		err := sm.StopService(context.Background(), name)
		if err != nil {
			return fmt.Errorf("failed to stop service %s before unregistering: %w", name, err)
		}
		sm.log(models.LevelInfo,
			"Service "+name+" stopped before unregistering",
			nil)
	}
	sm.mu.Lock()
	delete(sm.services, name)
	delete(sm.statuses, name)
	sm.mu.Unlock()

	sm.log(models.LevelInfo,
		"Service "+name+" unregistered successfully",
		nil)
	return nil
}

func (sm *ServiceManagerImpl) log(level LogLevel, message string, err error) {
	sm.config.Logger.LogEvent(
		context.Background(),
		LogMessage{
			Level:   level,
			Service: contracts.ServiceManagerName,
			Message: message,
			Err:     err,
		})
}

func (sm *ServiceManagerImpl) StartService(ctx context.Context, name string) error {
	var service Service
	var err error

	switch sm.GetServiceStatus(name) {
	case models.ServiceStatusRunning:
		return fmt.Errorf("service %s already running", name)
	case models.ServiceStatusStopped:
		sm.mu.RLock()
		service = sm.services[name]
		sm.mu.RUnlock()
	case models.ServiceStatusStarting:
		return fmt.Errorf("service %s is starting", name)
	case models.ServiceStatusStopping:
		return fmt.Errorf("service %s is stopping", name)
	default:
		return fmt.Errorf("service %s not registered", name)
	}

	sm.setServiceStatus(name, models.ServiceStatusStarting)
	for i := 0; i < sm.config.StartServiceAttemptCount; i++ {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, sm.config.StartServiceTimeout)
		err = service.Start(ctxWithTimeout)
		cancel()
		if err == nil {
			sm.log(models.LevelInfo,
				"Service "+name+" started successfully",
				nil)
			sm.setServiceStatus(name, models.ServiceStatusRunning)
			return nil
		}
		sm.log(models.LevelWarning,
			"Start attempt "+fmt.Sprintf("%d", i+1)+" for service "+name+" failed",
			err,
		)
	}
	sm.setServiceStatus(name, models.ServiceStatusStopped)
	sm.log(models.LevelWarning,
		"Service "+name+" failed to start after 3 attempts",
		err,
	)
	return nil
}

func (sm *ServiceManagerImpl) StartAllServices(ctx context.Context) {
	var notStarted []string
	var notStartedMutex sync.Mutex
	var wg sync.WaitGroup

	services := sm.snapshotServices()

	for name := range services {

		name := name

		wg.Add(1)

		go func(name string) {
			defer wg.Done()

			if err := sm.StartService(ctx, name); err != nil {

				notStartedMutex.Lock()
				notStarted = append(notStarted, name)
				notStartedMutex.Unlock()

				sm.log(models.LevelError,
					"Service "+name+" failed to start",
					err)

				return
			}
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
		sm.log(models.LevelWarning,
			"Trying to stop the service "+name+", but it is already stopped",
			nil)
		return nil
	case models.ServiceStatusStarting:
		return fmt.Errorf("service %s is starting", name)
	case models.ServiceStatusStopping:
		return fmt.Errorf("service %s is already stopping", name)
	default:
		return fmt.Errorf("service %s not registered", name)
	}

	sm.setServiceStatus(name, models.ServiceStatusStopping)

	done := make(chan error, 1)

	go func() {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, sm.config.StopServiceTimeout)
		done <- service.Shutdown(ctxWithTimeout)
		cancel()
	}()
	
	select {
	case err = <-done:
		if err == nil {
			sm.log(models.LevelInfo,
				"Service "+name+" stopped successfully",
				nil)
			sm.setServiceStatus(name, models.ServiceStatusStopped)
			return nil
		}
	case <-ctx.Done():
		sm.log(models.LevelWarning,
			"Service "+name+" shutdown timed out",
			nil)
		service.ForceKill()
		sm.log(models.LevelWarning,
			"Service "+name+" force killed",
			nil)
		sm.setServiceStatus(name, models.ServiceStatusStopped)
		return nil
	}
	return nil

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

			err := sm.StopService(ctx, name)

			if err != nil {
				notStoppedMutex.Lock()
				notStopped = append(notStopped, name)
				notStoppedMutex.Unlock()

				sm.log(models.LevelError,
					"Service "+name+" failed to stop",
					err)

				return
			}
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

func (sm *ServiceManagerImpl) RestartService(ctx context.Context, name string) error {

	if err := sm.StopService(ctx, name); err != nil {
		sm.log(models.LevelError,
			"Failed to stop service "+name+" while restarting",
			err)
		return err
	}

	if err := sm.StartService(ctx, name); err != nil {
		sm.log(models.LevelError,
			"Failed to start service "+name+" while restarting",
			err)
		return err
	}

	sm.log(models.LevelInfo,
		"Service "+name+" restarted successfully",
		nil)
	return nil
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
