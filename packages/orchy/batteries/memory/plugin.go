package memory

import (
	"errors"
	"fmt"

	"gz-ia/internal/features/memory"
	"gz-ia/packages/orchy/core"
	"gz-ia/packages/orchy/plugins"
)

type Option func(*MemoryPlugin)

func WithMemoryService(svc memory.Service) Option {
	return func(p *MemoryPlugin) {
		p.memoryService = svc
	}
}

type MemoryPlugin struct {
	plugins.BasePlugin
	projectDir      string
	memoryService   memory.Service
	saveTool        *MemorySaveTool
	searchTool      *MemorySearchTool
	listTool        *MemoryListTool
	consolidateTool *MemoryConsolidateTool
}

func NewMemoryPlugin(projectDir string, opts ...Option) *MemoryPlugin {
	p := &MemoryPlugin{
		BasePlugin: plugins.BasePlugin{
			PluginName:    "batteries.memory",
			PluginVersion: "0.0.1",
		},
		projectDir: projectDir,
	}
	for _, opt := range opts {
		opt(p)
	}

	if p.memoryService == nil {
		p.memoryService = memory.NewService(p.projectDir)
	}

	p.saveTool = NewMemorySaveTool(p.memoryService)
	p.searchTool = NewMemorySearchTool(p.memoryService)
	p.listTool = NewMemoryListTool(p.memoryService)
	p.consolidateTool = NewMemoryConsolidateTool(p.memoryService)

	return p
}

func (p *MemoryPlugin) Name() string {
	return "batteries.memory"
}

func (p *MemoryPlugin) Version() string {
	return "0.0.1"
}

func (p *MemoryPlugin) OnBoot(ctx *core.KernelContext) error {
	if ctx == nil {
		return errors.New("kernel context cannot be nil")
	}

	if _, err := ctx.RegisterTool(p.saveTool); err != nil {
		return fmt.Errorf("failed to register memory_save tool: %w", err)
	}
	if _, err := ctx.RegisterTool(p.searchTool); err != nil {
		return fmt.Errorf("failed to register memory_search tool: %w", err)
	}
	if _, err := ctx.RegisterTool(p.listTool); err != nil {
		return fmt.Errorf("failed to register memory_list tool: %w", err)
	}
	if _, err := ctx.RegisterTool(p.consolidateTool); err != nil {
		return fmt.Errorf("failed to register memory_consolidate tool: %w", err)
	}

	if ctx.Services() != nil && p.memoryService != nil {
		_ = ctx.Services().Register("memory.Service", p.memoryService)
	}

	return nil
}

func (p *MemoryPlugin) OnShutdown(ctx *core.KernelContext) error {
	return nil
}
