package orchy

import (
	"context"
	"io"

	"gz-ia/packages/orchy/core"
	"gz-ia/packages/orchy/events"
	"gz-ia/packages/orchy/mcp"
	"gz-ia/packages/orchy/plugins"
	"gz-ia/packages/orchy/tools"
)

// Re-exported types for ergonomic top-level use.
type (
	Kernel            = core.Kernel
	KernelContext     = core.KernelContext
	KernelState       = core.KernelState
	ServiceContainer  = core.ServiceContainer
	EventBus          = events.EventBus
	Disposable        = events.Disposable
	EventHandler      = events.EventHandler
	RequestHandler    = events.RequestHandler
	Tool              = tools.Tool
	ToolSchema        = tools.ToolSchema
	ToolStatus        = tools.ToolStatus
	ToolMetrics       = tools.ToolMetrics
	ToolProxy         = tools.ToolProxy
	ToolProxyOptions  = tools.ToolProxyOptions
	ToolRegistry      = tools.ToolRegistry
	Plugin            = plugins.Plugin
	BasePlugin        = plugins.BasePlugin
	PluginRegistry    = plugins.PluginRegistry
	ManifestGenerator = mcp.ManifestGenerator
	McpServer         = mcp.Server
	McpToolDefinition = mcp.McpToolDefinition
	ToolCatalogItem   = mcp.ToolCatalogItem
)

// Kernel lifecycle state constants.
const (
	KernelStateIdle         = core.KernelStateIdle
	KernelStateBooting      = core.KernelStateBooting
	KernelStateRunning      = core.KernelStateRunning
	KernelStateShuttingDown = core.KernelStateShuttingDown
	KernelStateStopped      = core.KernelStateStopped
)

// ToolStatus constants.
const (
	ToolStatusHealthy  = tools.ToolStatusHealthy
	ToolStatusDegraded = tools.ToolStatusDegraded
	ToolStatusDead     = tools.ToolStatusDead
)

// NewKernel constructs and returns a fully initialized Microkernel instance.
func NewKernel() *core.Kernel {
	return core.NewKernel()
}

// NewServiceContainer creates an empty IoC container.
func NewServiceContainer() *core.ServiceContainer {
	return core.NewServiceContainer()
}

// NewEventBus creates a new Pub/Sub and RPC EventBus.
func NewEventBus() *events.EventBus {
	return events.NewEventBus()
}

// NewToolRegistry creates an empty ToolRegistry.
func NewToolRegistry() *tools.ToolRegistry {
	return tools.NewToolRegistry()
}

// NewPluginRegistry creates an empty PluginRegistry.
func NewPluginRegistry() *plugins.PluginRegistry {
	return plugins.NewPluginRegistry()
}

// NewTool creates a Tool instance from a function.
func NewTool(
	name string,
	description string,
	schema tools.ToolSchema,
	fn func(ctx context.Context, input any) (any, error),
) tools.Tool {
	return &tools.FuncTool{
		ToolName:        name,
		ToolDescription: description,
		ToolSchema:      schema,
		Handler:         fn,
	}
}

// functionalPlugin implements plugins.Plugin via functional callbacks.
type functionalPlugin struct {
	plugins.BasePlugin
	bootFn     func(ctx *core.KernelContext) error
	shutdownFn func(ctx *core.KernelContext) error
}

func (p *functionalPlugin) OnBoot(ctx *core.KernelContext) error {
	if p.bootFn != nil {
		return p.bootFn(ctx)
	}
	return nil
}

func (p *functionalPlugin) OnShutdown(ctx *core.KernelContext) error {
	if p.shutdownFn != nil {
		return p.shutdownFn(ctx)
	}
	return nil
}

// NewPlugin creates a Plugin instance from callbacks.
func NewPlugin(
	name string,
	version string,
	onBoot func(ctx *core.KernelContext) error,
	onShutdown ...func(ctx *core.KernelContext) error,
) plugins.Plugin {
	var shutdownFn func(ctx *core.KernelContext) error
	if len(onShutdown) > 0 {
		shutdownFn = onShutdown[0]
	}

	return &functionalPlugin{
		BasePlugin: plugins.BasePlugin{
			PluginName:    name,
			PluginVersion: version,
		},
		bootFn:     onBoot,
		shutdownFn: shutdownFn,
	}
}

// NewManifestGenerator constructs a ManifestGenerator for a ToolRegistry.
func NewManifestGenerator(toolRegistry *tools.ToolRegistry, pluginProviders ...func() []mcp.PluginDescriptor) *mcp.ManifestGenerator {
	return mcp.NewManifestGenerator(toolRegistry, pluginProviders...)
}

// NewMcpServer wraps a microkernel Kernel in an MCP stdio JSON-RPC 2.0 server.
func NewMcpServer(kernel *core.Kernel, in io.Reader, out io.Writer) *mcp.Server {
	return mcp.NewServer(kernel.GetIntrospector(), kernel.GetContext(), in, out)
}
