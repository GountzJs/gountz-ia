package core

import (
	"context"
	"fmt"

	"gz-ia/packages/orchy/events"
	"gz-ia/packages/orchy/tools"
)

// KernelContext is the central execution context passed to plugins, tools, and kernel operations.
type KernelContext struct {
	services *ServiceContainer
	eventBus *events.EventBus
	tools    *tools.ToolRegistry
}

// NewKernelContext creates a new KernelContext instance.
func NewKernelContext(services *ServiceContainer, eventBus *events.EventBus, toolRegistry *tools.ToolRegistry) *KernelContext {
	return &KernelContext{
		services: services,
		eventBus: eventBus,
		tools:    toolRegistry,
	}
}

// RegisterTool registers a tool into the kernel tool registry.
func (c *KernelContext) RegisterTool(tool tools.Tool, opts ...tools.ToolProxyOptions) (*tools.ToolProxy, error) {
	return c.tools.Register(tool, opts...)
}

// ExecuteTool invokes a registered tool by name with circuit breaker protection.
func (c *KernelContext) ExecuteTool(ctx context.Context, name string, input any) (any, error) {
	proxy, exists := c.tools.Get(name)
	if !exists {
		return nil, fmt.Errorf("[KernelContext] tool %q not found in registry", name)
	}
	return proxy.Execute(ctx, input)
}

// RegisterService adds a service to the dependency injection container.
func (c *KernelContext) RegisterService(name string, service any) error {
	return c.services.Register(name, service)
}

// GetService retrieves a service from the dependency injection container.
func (c *KernelContext) GetService(name string) (any, error) {
	return c.services.Get(name)
}

// Emit broadcasts an event over the event bus.
func (c *KernelContext) Emit(ctx context.Context, event string, payload any) error {
	return c.eventBus.Publish(ctx, event, payload)
}

// On subscribes a handler to a specific event topic.
func (c *KernelContext) On(event string, handler events.EventHandler) events.Disposable {
	return c.eventBus.Subscribe(event, handler)
}

// Request sends an RPC request across the event bus and awaits a response.
func (c *KernelContext) Request(ctx context.Context, topic string, payload any) (any, error) {
	return c.eventBus.Request(ctx, topic, payload)
}

// Respond registers an RPC request-response handler on a specific topic.
func (c *KernelContext) Respond(topic string, handler events.RequestHandler) (events.Disposable, error) {
	return c.eventBus.Respond(topic, handler)
}

// Services returns the underlying ServiceContainer.
func (c *KernelContext) Services() *ServiceContainer {
	return c.services
}

// EventBus returns the underlying EventBus.
func (c *KernelContext) EventBus() *events.EventBus {
	return c.eventBus
}

// Tools returns the underlying ToolRegistry.
func (c *KernelContext) Tools() *tools.ToolRegistry {
	return c.tools
}
