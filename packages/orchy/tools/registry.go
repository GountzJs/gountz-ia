package tools

import (
	"fmt"
	"sync"
)

// ToolRegistry maintains a thread-safe registry of tools wrapped with ToolProxy.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*ToolProxy
	order []string
}

// NewToolRegistry creates an empty ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*ToolProxy),
		order: make([]string, 0),
	}
}

// Register registers a tool or existing ToolProxy into the registry.
// Returns an error if a tool with the same name is already registered.
func (r *ToolRegistry) Register(tool Tool, opts ...ToolProxyOptions) (*ToolProxy, error) {
	if tool == nil {
		return nil, fmt.Errorf("[ToolRegistry] cannot register nil tool")
	}

	name := tool.Name()
	if name == "" {
		return nil, fmt.Errorf("[ToolRegistry] tool name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return nil, fmt.Errorf("[ToolRegistry] tool with name %q is already registered", name)
	}

	proxy, ok := tool.(*ToolProxy)
	if !ok {
		proxy = NewToolProxy(tool, opts...)
	}

	r.tools[name] = proxy
	r.order = append(r.order, name)
	return proxy, nil
}

// Get retrieves a ToolProxy by name.
func (r *ToolRegistry) Get(name string) (*ToolProxy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	proxy, exists := r.tools[name]
	return proxy, exists
}

// Has checks if a tool with the given name is registered.
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// Remove unregisters a tool by name. Returns true if the tool was removed.
func (r *ToolRegistry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return false
	}

	delete(r.tools, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return true
}

// List returns a list of all registered tool proxies in registration order.
func (r *ToolRegistry) List() []*ToolProxy {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]*ToolProxy, 0, len(r.order))
	for _, name := range r.order {
		if p, ok := r.tools[name]; ok {
			res = append(res, p)
		}
	}
	return res
}

// GetHealthyTools returns all registered tools whose status is not DEAD (e.g. HEALTHY or DEGRADED).
func (r *ToolRegistry) GetHealthyTools() []*ToolProxy {
	all := r.List()
	healthy := make([]*ToolProxy, 0, len(all))
	for _, t := range all {
		if t.Status() != ToolStatusDead {
			healthy = append(healthy, t)
		}
	}
	return healthy
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Clear removes all tools from the registry.
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]*ToolProxy)
	r.order = make([]string, 0)
}
