package plugins

import (
	"fmt"
	"sync"

	"gz-ia/packages/orchy/core"
)

// PluginRegistry manages registered plugins and coordinates their lifecycle execution.
type PluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	order   []string
}

// NewPluginRegistry creates an empty PluginRegistry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
		order:   make([]string, 0),
	}
}

// Register registers a plugin. Returns an error if a plugin with the same name is already registered.
func (r *PluginRegistry) Register(plugin Plugin) error {
	if plugin == nil {
		return fmt.Errorf("[PluginRegistry] cannot register nil plugin")
	}

	name := plugin.Name()
	if name == "" {
		return fmt.Errorf("[PluginRegistry] plugin name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("[PluginRegistry] plugin %q is already registered", name)
	}

	r.plugins[name] = plugin
	r.order = append(r.order, name)
	return nil
}

// Get retrieves a plugin by name.
func (r *PluginRegistry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	return plugin, exists
}

// Has checks if a plugin with the given name is registered.
func (r *PluginRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.plugins[name]
	return exists
}

// Remove unregisters a plugin by name.
func (r *PluginRegistry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return false
	}

	delete(r.plugins, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return true
}

// List returns all registered plugins in registration order.
func (r *PluginRegistry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]Plugin, 0, len(r.order))
	for _, name := range r.order {
		if p, ok := r.plugins[name]; ok {
			res = append(res, p)
		}
	}
	return res
}

// Count returns the number of registered plugins.
func (r *PluginRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.plugins)
}

// Clear removes all registered plugins.
func (r *PluginRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins = make(map[string]Plugin)
	r.order = make([]string, 0)
}

// BootAll executes OnBoot on all registered plugins in order.
func (r *PluginRegistry) BootAll(ctx *core.KernelContext) error {
	plugins := r.List()
	for _, p := range plugins {
		if err := p.OnBoot(ctx); err != nil {
			return fmt.Errorf("[PluginRegistry] failed to boot plugin %q: %w", p.Name(), err)
		}
	}
	return nil
}

// ShutdownAll executes OnShutdown on all registered plugins in reverse registration order.
func (r *PluginRegistry) ShutdownAll(ctx *core.KernelContext) error {
	plugins := r.List()
	var firstErr error
	for i := len(plugins) - 1; i >= 0; i-- {
		p := plugins[i]
		if err := p.OnShutdown(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("[PluginRegistry] error shutting down plugin %q: %w", p.Name(), err)
		}
	}
	return firstErr
}
