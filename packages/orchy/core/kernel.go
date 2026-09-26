package core

import (
	"context"
	"fmt"
	"sync"

	"gz-ia/packages/orchy/events"
	"gz-ia/packages/orchy/mcp"
	"gz-ia/packages/orchy/tools"
)

// KernelState represents the lifecycle status of the Kernel.
type KernelState string

const (
	KernelStateIdle         KernelState = "IDLE"
	KernelStateBooting      KernelState = "BOOTING"
	KernelStateRunning      KernelState = "RUNNING"
	KernelStateShuttingDown KernelState = "SHUTTING_DOWN"
	KernelStateStopped      KernelState = "STOPPED"
)

// Plugin defines the lifecycle interface for kernel plugins.
type Plugin interface {
	Name() string
	Version() string
	OnBoot(ctx *KernelContext) error
	OnShutdown(ctx *KernelContext) error
}

// Kernel is the central microkernel orchestrating services, events, tools, and plugins.
type Kernel struct {
	mu           sync.RWMutex
	state        KernelState
	services     *ServiceContainer
	eventBus     *events.EventBus
	tools        *tools.ToolRegistry
	plugins      []Plugin
	pluginMap    map[string]Plugin
	context      *KernelContext
	introspector *mcp.ManifestGenerator
}

// NewKernel initializes a new Kernel instance.
func NewKernel() *Kernel {
	sc := NewServiceContainer()
	eb := events.NewEventBus()
	tr := tools.NewToolRegistry()
	ctx := NewKernelContext(sc, eb, tr)

	k := &Kernel{
		state:     KernelStateIdle,
		services:  sc,
		eventBus:  eb,
		tools:     tr,
		plugins:   make([]Plugin, 0),
		pluginMap: make(map[string]Plugin),
		context:   ctx,
	}

	// Wire introspector with tool registry and plugin provider
	k.introspector = mcp.NewManifestGenerator(tr, func() []mcp.PluginDescriptor {
		k.mu.RLock()
		defer k.mu.RUnlock()
		descriptors := make([]mcp.PluginDescriptor, len(k.plugins))
		for i, p := range k.plugins {
			descriptors[i] = p
		}
		return descriptors
	})

	return k
}

// State returns the current lifecycle state of the Kernel.
func (k *Kernel) State() KernelState {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.state
}

// IsRunning returns true if the kernel is in RUNNING state.
func (k *Kernel) IsRunning() bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.state == KernelStateRunning
}

// GetContext returns the KernelContext.
func (k *Kernel) GetContext() *KernelContext {
	return k.context
}

// GetIntrospector returns the ManifestGenerator introspector for tools and plugins.
func (k *Kernel) GetIntrospector() *mcp.ManifestGenerator {
	return k.introspector
}

// Services returns the kernel ServiceContainer.
func (k *Kernel) Services() *ServiceContainer {
	return k.services
}

// EventBus returns the kernel EventBus.
func (k *Kernel) EventBus() *events.EventBus {
	return k.eventBus
}

// Tools returns the kernel ToolRegistry.
func (k *Kernel) Tools() *tools.ToolRegistry {
	return k.tools
}

// Use registers a plugin with the Kernel.
// If the Kernel is already RUNNING, the plugin is booted immediately.
func (k *Kernel) Use(plugin Plugin) error {
	if plugin == nil {
		return fmt.Errorf("[Kernel] cannot register nil plugin")
	}

	name := plugin.Name()
	if name == "" {
		return fmt.Errorf("[Kernel] plugin name cannot be empty")
	}

	k.mu.Lock()
	if _, exists := k.pluginMap[name]; exists {
		k.mu.Unlock()
		return fmt.Errorf("[Kernel] plugin %q is already registered", name)
	}

	k.plugins = append(k.plugins, plugin)
	k.pluginMap[name] = plugin
	currentState := k.state
	k.mu.Unlock()

	// If already running, boot this plugin immediately
	if currentState == KernelStateRunning {
		if err := plugin.OnBoot(k.context); err != nil {
			return fmt.Errorf("[Kernel] error booting plugin %q: %w", name, err)
		}
	}

	return nil
}

// RegisterTool registers a tool into the Kernel's tool registry.
func (k *Kernel) RegisterTool(tool tools.Tool, opts ...tools.ToolProxyOptions) (*tools.ToolProxy, error) {
	return k.tools.Register(tool, opts...)
}

// Boot transitions the kernel to RUNNING and boots all registered plugins.
func (k *Kernel) Boot(ctx context.Context) error {
	k.mu.Lock()
	if k.state == KernelStateRunning || k.state == KernelStateBooting {
		k.mu.Unlock()
		return nil
	}
	k.state = KernelStateBooting
	k.mu.Unlock()

	if err := k.eventBus.Publish(ctx, "kernel:booting", map[string]any{"kernel": k}); err != nil {
		// Log or proceed
	}

	k.mu.RLock()
	pluginsToBoot := make([]Plugin, len(k.plugins))
	copy(pluginsToBoot, k.plugins)
	k.mu.RUnlock()

	for _, p := range pluginsToBoot {
		if err := ctx.Err(); err != nil {
			k.mu.Lock()
			k.state = KernelStateStopped
			k.mu.Unlock()
			return err
		}

		if err := p.OnBoot(k.context); err != nil {
			k.mu.Lock()
			k.state = KernelStateStopped
			k.mu.Unlock()
			return fmt.Errorf("[Kernel] failed to boot plugin %q: %w", p.Name(), err)
		}
	}

	k.mu.Lock()
	k.state = KernelStateRunning
	k.mu.Unlock()

	_ = k.eventBus.Publish(ctx, "kernel:ready", map[string]any{"kernel": k})
	return nil
}

// Shutdown transitions the kernel to STOPPED, shutting down plugins in reverse order.
func (k *Kernel) Shutdown(ctx context.Context) error {
	k.mu.Lock()
	if k.state != KernelStateRunning && k.state != KernelStateBooting {
		k.mu.Unlock()
		return nil
	}
	k.state = KernelStateShuttingDown
	k.mu.Unlock()

	_ = k.eventBus.Publish(ctx, "kernel:shutting_down", map[string]any{"kernel": k})

	k.mu.RLock()
	pluginsToShutdown := make([]Plugin, len(k.plugins))
	copy(pluginsToShutdown, k.plugins)
	k.mu.RUnlock()

	var firstErr error
	// Shutdown plugins in reverse registration order
	for i := len(pluginsToShutdown) - 1; i >= 0; i-- {
		p := pluginsToShutdown[i]
		if err := p.OnShutdown(k.context); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("[Kernel] error shutting down plugin %q: %w", p.Name(), err)
		}
	}

	k.eventBus.Clear()

	k.mu.Lock()
	k.state = KernelStateStopped
	k.mu.Unlock()

	return firstErr
}
