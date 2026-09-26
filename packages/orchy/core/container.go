package core

import (
	"fmt"
	"sync"
)

// ServiceContainer provides a thread-safe Inversion of Control (IoC) dependency injection container.
type ServiceContainer struct {
	mu       sync.RWMutex
	services map[string]any
}

// NewServiceContainer creates an empty ServiceContainer.
func NewServiceContainer() *ServiceContainer {
	return &ServiceContainer{
		services: make(map[string]any),
	}
}

// Register registers a named service. Returns an error if the service is already registered.
func (c *ServiceContainer) Register(name string, service any) error {
	if name == "" {
		return fmt.Errorf("[ServiceContainer] service name cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.services[name]; exists {
		return fmt.Errorf("[ServiceContainer] service %q is already registered", name)
	}

	c.services[name] = service
	return nil
}

// Get retrieves a service by name. Returns an error if the service is not found.
func (c *ServiceContainer) Get(name string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service, exists := c.services[name]
	if !exists {
		return nil, fmt.Errorf("[ServiceContainer] service %q is not registered", name)
	}
	return service, nil
}

// TryGet attempts to retrieve a service by name without returning an error.
func (c *ServiceContainer) TryGet(name string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service, exists := c.services[name]
	return service, exists
}

// Has checks whether a service with the given name exists in the container.
func (c *ServiceContainer) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.services[name]
	return exists
}

// Remove deletes a service from the container. Returns true if the service was removed.
func (c *ServiceContainer) Remove(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.services[name]; !exists {
		return false
	}
	delete(c.services, name)
	return true
}

// List returns the names of all registered services.
func (c *ServiceContainer) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}
	return names
}

// Clear removes all services from the container.
func (c *ServiceContainer) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.services = make(map[string]any)
}
