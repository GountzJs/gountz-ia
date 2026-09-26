package worktree

import (
	"encoding/json"
	"errors"
	"fmt"

	"gz-ia/internal/features/session"
	"gz-ia/internal/features/workspace"
	"gz-ia/packages/orchy/core"
	"gz-ia/packages/orchy/plugins"
)

// Option defines a functional option for configuring WorktreePlugin.
type Option func(*WorktreePlugin)

// WithSessionService injects a custom session.Service into WorktreePlugin.
func WithSessionService(svc session.Service) Option {
	return func(p *WorktreePlugin) {
		p.sessionService = svc
	}
}

// WithWorkspaceProvider injects a custom workspace.Provider into WorktreePlugin.
func WithWorkspaceProvider(wp workspace.Provider) Option {
	return func(p *WorktreePlugin) {
		p.workspaceProvider = wp
	}
}

// WithBaseDir sets the base directory for WorktreePlugin.
func WithBaseDir(baseDir string) Option {
	return func(p *WorktreePlugin) {
		p.baseDir = baseDir
	}
}

// WithAllowAgentGet allows exposing the worktree_get tool to the agent via MCP.
// WARNING: By default this is false. Merging code into the active workspace is a strictly human action.
func WithAllowAgentGet(allow bool) Option {
	return func(p *WorktreePlugin) {
		p.allowAgentGet = allow
	}
}

// WorktreePlugin integrates gz-ia session worktree operations into Orchy microkernel.
type WorktreePlugin struct {
	plugins.BasePlugin
	baseDir           string
	sessionService    session.Service
	workspaceProvider workspace.Provider
	allowAgentGet     bool
	readTool          *WorktreeReadTool
	getTool           *WorktreeGetTool
}

// NewWorktreePlugin creates a new WorktreePlugin with optional configurations.
func NewWorktreePlugin(baseDir string, opts ...Option) *WorktreePlugin {
	p := &WorktreePlugin{
		BasePlugin: plugins.BasePlugin{
			PluginName:    "batteries.worktree",
			PluginVersion: "0.0.1",
		},
		baseDir: baseDir,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.sessionService == nil {
		var sessionOpts []session.Option
		if p.workspaceProvider != nil {
			sessionOpts = append(sessionOpts, session.WithWorkspace(p.workspaceProvider))
		}
		p.sessionService = session.NewService(p.baseDir, sessionOpts...)
	}

	p.readTool = NewWorktreeReadTool(p.sessionService)
	p.getTool = NewWorktreeGetTool(p.sessionService)

	return p
}

// Name returns the plugin identifier.
func (p *WorktreePlugin) Name() string {
	return "batteries.worktree"
}

// Version returns the plugin version.
func (p *WorktreePlugin) Version() string {
	return "0.0.1"
}

// OnBoot registers worktree tools in the kernel.
// By default, only worktree_read is registered so agents can inspect their progress.
// worktree_get is strictly reserved for human CLI invocation unless WithAllowAgentGet(true) is explicitly passed.
func (p *WorktreePlugin) OnBoot(ctx *core.KernelContext) error {
	if ctx == nil {
		return errors.New("kernel context cannot be nil")
	}

	if _, err := ctx.RegisterTool(p.readTool); err != nil {
		return fmt.Errorf("failed to register worktree_read tool: %w", err)
	}

	if p.allowAgentGet && p.getTool != nil {
		if _, err := ctx.RegisterTool(p.getTool); err != nil {
			return fmt.Errorf("failed to register worktree_get tool: %w", err)
		}
	}

	return nil
}

// OnShutdown cleanly shuts down the plugin.
func (p *WorktreePlugin) OnShutdown(ctx *core.KernelContext) error {
	return nil
}

// GetReadTool returns the initialized WorktreeReadTool instance.
func (p *WorktreePlugin) GetReadTool() *WorktreeReadTool {
	return p.readTool
}

// GetGetTool returns the initialized WorktreeGetTool instance.
func (p *WorktreePlugin) GetGetTool() *WorktreeGetTool {
	return p.getTool
}

// GetSessionService returns the configured session.Service.
func (p *WorktreePlugin) GetSessionService() session.Service {
	return p.sessionService
}

func decodeInput(input any, target any) error {
	if input == nil {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
