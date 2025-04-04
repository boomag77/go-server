package models

type ServiceStatus = string

const (
	ServiceStatusRunning  ServiceStatus = "RUNNING"
	ServiceStatusStopped  ServiceStatus = "STOPPED"
	ServiceStatusStarting ServiceStatus = "STARTING"
	ServiceStatusStopping ServiceStatus = "STOPPING"
)
