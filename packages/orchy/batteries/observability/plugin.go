package observability

import (
	"errors"
	"fmt"

	"gz-ia/internal/features/logger"
	"gz-ia/packages/orchy/core"
	"gz-ia/packages/orchy/plugins"
)

// Option define opciones funcionales para configurar ObservabilityPlugin.
type Option func(*ObservabilityPlugin)

// WithLogger inyecta un logger.Service personalizado en el plugin.
func WithLogger(l logger.Service) Option {
	return func(p *ObservabilityPlugin) {
		p.loggerService = l
	}
}

// WithSessionID configura el sessionID por defecto para las operaciones de log del plugin.
func WithSessionID(sessionID string) Option {
	return func(p *ObservabilityPlugin) {
		p.sessionID = sessionID
	}
}

// WithBaseDir configura el directorio base para resolver el logger.Service por defecto.
func WithBaseDir(baseDir string) Option {
	return func(p *ObservabilityPlugin) {
		p.baseDir = baseDir
	}
}

// ObservabilityPlugin integra capacidades de observabilidad y registro de eventos agénticos en Orchy.
type ObservabilityPlugin struct {
	plugins.BasePlugin
	baseDir       string
	sessionID     string
	loggerService logger.Service
	logTool       *SessionLogTool
}

// NewObservabilityPlugin crea un nuevo ObservabilityPlugin configurado.
func NewObservabilityPlugin(baseDir string, opts ...Option) *ObservabilityPlugin {
	p := &ObservabilityPlugin{
		BasePlugin: plugins.BasePlugin{
			PluginName:    "batteries.observability",
			PluginVersion: "0.0.1",
		},
		baseDir: baseDir,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.loggerService == nil {
		p.loggerService = logger.NewService(p.baseDir)
	}

	p.logTool = NewSessionLogTool(p.loggerService, WithDefaultSessionID(p.sessionID))

	return p
}

// Name retorna el identificador del plugin.
func (p *ObservabilityPlugin) Name() string {
	return "batteries.observability"
}

// Version retorna la versión del plugin.
func (p *ObservabilityPlugin) Version() string {
	return "0.0.1"
}

// OnBoot registra la herramienta session_log en el microkernel Orchy.
func (p *ObservabilityPlugin) OnBoot(ctx *core.KernelContext) error {
	if ctx == nil {
		return errors.New("kernel context cannot be nil")
	}

	if _, err := ctx.RegisterTool(p.logTool); err != nil {
		return fmt.Errorf("failed to register session_log tool: %w", err)
	}

	if ctx.Services() != nil && p.loggerService != nil {
		_ = ctx.Services().Register("logger.Service", p.loggerService)
	}

	return nil
}

// OnShutdown finaliza limpiamente el plugin.
func (p *ObservabilityPlugin) OnShutdown(ctx *core.KernelContext) error {
	return nil
}

// GetLogTool retorna la instancia inicializada de SessionLogTool.
func (p *ObservabilityPlugin) GetLogTool() *SessionLogTool {
	return p.logTool
}

// GetLoggerService retorna el logger.Service configurado.
func (p *ObservabilityPlugin) GetLoggerService() logger.Service {
	return p.loggerService
}
