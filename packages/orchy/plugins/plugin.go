package plugins

import (
	"gz-ia/packages/orchy/core"
)

// Plugin defines the microkernel plugin interface.
type Plugin interface {
	Name() string
	Version() string
	OnBoot(ctx *core.KernelContext) error
	OnShutdown(ctx *core.KernelContext) error
}

// BasePlugin provides a default implementation of Plugin that can be embedded in custom plugins.
type BasePlugin struct {
	PluginName    string
	PluginVersion string
}

// Name returns the plugin name.
func (b *BasePlugin) Name() string {
	return b.PluginName
}

// Version returns the plugin version, defaulting to "1.0.0" if unspecified.
func (b *BasePlugin) Version() string {
	if b.PluginVersion == "" {
		return "1.0.0"
	}
	return b.PluginVersion
}

// OnBoot default hook doing nothing.
func (b *BasePlugin) OnBoot(ctx *core.KernelContext) error {
	return nil
}

// OnShutdown default hook doing nothing.
func (b *BasePlugin) OnShutdown(ctx *core.KernelContext) error {
	return nil
}
