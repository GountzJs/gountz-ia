package session

import (
	"context"
	"errors"
	"fmt"
	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/profile"
	"gz-ia/internal/features/tooling"
	"gz-ia/internal/features/vault"
	"gz-ia/internal/features/workspace"
	"os"
	"os/exec"
	"time"
)

// PermissionLevel define el grado de permisos otorgados a agy en la sesión.
type PermissionLevel string

const (
	// PermissionReadOnly ejecuta agy en modo plan (solo lectura sobre el proyecto, permitiendo MCPs).
	PermissionReadOnly PermissionLevel = "readonly"

	// PermissionSupervised solicita confirmación interactiva [y/N] al usuario antes de modificar código o ejecutar comandos.
	PermissionSupervised PermissionLevel = "supervised"

	// PermissionAutonomous auto-aprueba herramientas y comandos dentro del workspace de trabajo.
	PermissionAutonomous PermissionLevel = "autonomous"
)

// Config contiene los parámetros para iniciar una sesión de chat con el agente.
type Config struct {
	ID              string
	Provider        string
	WorkingDir      string
	InitialPrompt   string
	PermissionLevel PermissionLevel
	Resume          bool
	BinaryPath      string
	IsIsolated      bool
	WorktreeDir     string
	BranchName      string
	Profiles        []string
}

// DefaultConfig retorna la configuración inicial básica asignando el primer proveedor disponible o "agy".
func DefaultConfig() Config {
	cwd, _ := os.Getwd()
	provider := "agy"
	if first := FirstAvailableDriver(); first != nil {
		provider = first.ID()
	}
	binary := provider
	if d, err := GetDriver(provider); err == nil {
		binary = d.BinaryName()
	}
	return Config{
		ID:              GenerateID(),
		Provider:        provider,
		WorkingDir:      cwd,
		InitialPrompt:   "",
		PermissionLevel: PermissionSupervised,
		Resume:          false,
		BinaryPath:      binary,
	}
}

// ResolveBinaryPath localiza el ejecutable en el PATH del sistema de forma estándar y portable.
func ResolveBinaryPath(binary string) (string, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("ejecutable '%s' no encontrado en el PATH: %w", binary, err)
	}
	return path, nil
}

// BuildArgs construye los argumentos de línea de comandos delegando en el driver correspondiente al proveedor.
func BuildArgs(cfg Config) ([]string, error) {
	if cfg.WorkingDir == "" {
		return nil, errors.New("working directory no puede estar vacío")
	}

	providerID := cfg.Provider
	if providerID == "" {
		providerID = "agy"
	}

	driver, err := GetDriver(providerID)
	if err != nil {
		return nil, err
	}

	if !driver.IsAvailable() {
		return nil, fmt.Errorf("el agente '%s' (%s) no está disponible en el PATH.\nInstalación: %s", driver.DisplayName(), driver.BinaryName(), driver.InstallHint())
	}

	return driver.BuildArgs(cfg)
}

// Runner es la interfaz para ejecutar procesos conectando la terminal e informando del PID.
type Runner interface {
	Run(ctx context.Context, binary string, args []string, dir string, onStart func(pid int)) (exitCode int, err error)
}

// EnvSetter define el método opcional para runners que admiten inyección de variables de entorno.
type EnvSetter interface {
	SetEnv(env []string)
}

// OSRunner ejecuta el proceso en el sistema operativo conectando la terminal interactiva (TTY).
type OSRunner struct {
	Env []string
}

// SetEnv asigna las variables de entorno formateadas ("CLAVE=VALOR") para el proceso.
func (r *OSRunner) SetEnv(env []string) {
	r.Env = env
}

// Run ejecuta agy conectando stdin, stdout y stderr e informando del PID cuando el proceso inicia.
func (r *OSRunner) Run(ctx context.Context, binary string, args []string, dir string, onStart func(pid int)) (int, error) {
	binPath, err := ResolveBinaryPath(binary)
	if err != nil {
		return -1, err
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return killProcessGroup(cmd.Process.Pid)
		}
		return nil
	}
	restoreTTY := configureSysProcAttr(cmd)
	defer restoreTTY()
	if len(r.Env) > 0 {
		cmd.Env = r.Env
	}

	if err := cmd.Start(); err != nil {
		return -1, err
	}

	if onStart != nil && cmd.Process != nil {
		onStart(cmd.Process.Pid)
	}

	waitErr := cmd.Wait()
	restoreTTY()
	exitCode := 0
	if waitErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return exitCode, waitErr
}

// Session representa la sesión de chat con agy y su ciclo de vida.
type Session struct {
	Config    Config
	Runner    Runner
	Store     Store
	Workspace workspace.Provider
	Logger    logger.Service
	Profile   profile.Service
	Vault     vault.Service
	Tooling   tooling.Service
}

// New crea una nueva instancia de Session con runner y store inyectados.
func New(cfg Config, runner Runner, store ...Store) *Session {
	var r Runner = runner
	if r == nil {
		r = &OSRunner{}
	}

	var s Store
	if len(store) > 0 && store[0] != nil {
		s = store[0]
	} else {
		s = DefaultFileStore(cfg.WorkingDir)
	}

	if cfg.ID == "" {
		cfg.ID = GenerateID()
	}

	return &Session{
		Config:    cfg,
		Runner:    r,
		Store:     s,
		Workspace: workspace.NewDefaultProvider(),
	}
}

// WithWorkspace permite inyectar un proveedor de workspace personalizado.
func (s *Session) WithWorkspace(ws workspace.Provider) *Session {
	s.Workspace = ws
	return s
}

// WithLogger permite inyectar un servicio de logger para observabilidad de eventos de sesión.
func (s *Session) WithLogger(l logger.Service) *Session {
	s.Logger = l
	return s
}

// WithProfile permite inyectar un servicio de perfiles agénticos.
func (s *Session) WithProfile(p profile.Service) *Session {
	s.Profile = p
	return s
}

// WithVault permite inyectar un servicio de vault para cargar variables de entorno seguras.
func (s *Session) WithVault(v vault.Service) *Session {
	s.Vault = v
	return s
}

// WithTooling permite inyectar un servicio de tooling modular.
func (s *Session) WithTooling(t tooling.Service) *Session {
	s.Tooling = t
	return s
}

// Start lanza la sesión de chat con agy registrando su estado en el Store.
func (s *Session) Start(ctx context.Context) error {
	args, err := BuildArgs(s.Config)
	if err != nil {
		return fmt.Errorf("error al construir argumentos de sesión: %w", err)
	}

	prov := s.Config.Provider
	if prov == "" {
		prov = "agy"
		s.Config.Provider = prov
	}

	binary := s.Config.BinaryPath
	if binary == "" {
		if d, err := GetDriver(prov); err == nil {
			binary = d.BinaryName()
		} else {
			binary = "agy"
		}
	}

	wsProvider := s.Workspace
	if wsProvider == nil {
		wsProvider = workspace.NewDefaultProvider()
	}

	var ws *workspace.Workspace
	if s.Config.Resume && s.Config.IsIsolated && s.Config.WorktreeDir != "" {
		ws = &workspace.Workspace{
			WorkingDir:  s.Config.WorkingDir,
			TargetDir:   s.Config.WorktreeDir,
			IsIsolated:  true,
			WorktreeDir: s.Config.WorktreeDir,
			BranchName:  s.Config.BranchName,
		}
	} else {
		var prepErr error
		ws, prepErr = wsProvider.Prepare(ctx, s.Config.ID, s.Config.WorkingDir)
		if prepErr != nil {
			return fmt.Errorf("error al preparar espacio de trabajo: %w", prepErr)
		}
	}

	// Proyección modular de directivas y tooling (unificado bajo el proyector de Tooling)
	if s.Tooling != nil && len(s.Config.Profiles) > 0 {
		var activeToolkitIDs []string
		mcpServers := make(map[string]any)
		envVars := make(map[string]string)

		for _, profName := range s.Config.Profiles {
			if profCfg, err := s.Tooling.GetProfile(ctx, profName); err == nil && profCfg != nil {
				activeToolkitIDs = append(activeToolkitIDs, profCfg.Toolkits...)
				for k, v := range profCfg.MCPServers {
					mcpServers[k] = v
				}
				for k, v := range profCfg.Env {
					envVars[k] = v
				}
			}
		}

		// Si s.Profile también tiene definiciones de perfil (ej. MCP servers o directivas), integrarlos
		if s.Profile != nil {
			if profComposed, err := s.Profile.Compose(ctx, s.Config.Profiles); err == nil && profComposed != nil {
				for k, v := range profComposed.MCPServers {
					mcpServers[k] = v
				}
				for k, v := range profComposed.Env {
					envVars[k] = v
				}
			}
		}

		composedTooling, err := s.Tooling.ComposeToolkits(ctx, activeToolkitIDs)
		if err != nil {
			return fmt.Errorf("error al componer toolkits en tooling: %w", err)
		}
		if composedTooling != nil {
			for k, v := range mcpServers {
				composedTooling.MCPServers[k] = v
			}
			for k, v := range envVars {
				composedTooling.Env[k] = v
			}
			if projErr := s.Tooling.ProjectIntoWorktree(ctx, ws.TargetDir, composedTooling, s.Config.ID, ws.WorkingDir); projErr != nil {
				return fmt.Errorf("error al proyectar tooling en el worktree: %w", projErr)
			}
		}
	}

	startTime := time.Now()
	record := &SessionRecord{
		ID:              s.Config.ID,
		Provider:        s.Config.Provider,
		Status:          StatusRunning,
		PermissionLevel: s.Config.PermissionLevel,
		WorkingDir:      s.Config.WorkingDir,
		InitialPrompt:   s.Config.InitialPrompt,
		StartedAt:       startTime,
		IsIsolated:      ws.IsIsolated,
		WorktreeDir:     ws.WorktreeDir,
		BranchName:      ws.BranchName,
		Profiles:        s.Config.Profiles,
	}

	// Persistir estado inicial de ejecución
	_ = s.Store.Save(record)

	if s.Logger != nil {
		action := "Sesión de chat iniciada"
		if s.Config.Resume {
			action = "Sesión de chat reanudada"
		}
		_ = s.Logger.Emit(ctx, &logger.Event{
			SessionID: s.Config.ID,
			AgentID:   "orchestrator",
			Role:      "orchestrator",
			Action:    action,
			Stage:     logger.StagePending,
			Status:    nil,
		})
	}

	onStart := func(pid int) {
		record.PID = pid
		_ = s.Store.Save(record)
	}

	// Inyectar variables de entorno del Vault si está disponible
	if s.Vault != nil {
		if envSlice, err := s.Vault.LoadMergedEnvSlice(ctx); err == nil && len(envSlice) > 0 {
			if setter, ok := s.Runner.(EnvSetter); ok {
				setter.SetEnv(envSlice)
			}
		}
	}

	exitCode, runErr := s.Runner.Run(ctx, binary, args, ws.TargetDir, onStart)

	// Comprobar si fue marcada como killed externamente
	if latest, getErr := s.Store.Get(s.Config.ID); getErr == nil && latest != nil {
		if latest.Status == StatusKilled {
			return runErr
		}
	}

	endTime := time.Now()
	record.FinishedAt = &endTime
	record.DurationMs = endTime.Sub(startTime).Milliseconds()
	record.ExitCode = exitCode

	if runErr != nil {
		record.Status = StatusFailed
	} else {
		record.Status = StatusCompleted
	}

	_ = s.Store.Save(record)

	if s.Logger != nil {
		dur := record.DurationMs
		if runErr != nil {
			st := logger.StatusFailed
			_ = s.Logger.Emit(ctx, &logger.Event{
				SessionID:  s.Config.ID,
				AgentID:    "orchestrator",
				Role:       "orchestrator",
				Action:     "Sesión finalizada con fallas",
				Stage:      logger.StageFinish,
				Status:     &st,
				DurationMs: &dur,
				Error:      runErr.Error(),
			})
		} else {
			st := logger.StatusOK
			_ = s.Logger.Emit(ctx, &logger.Event{
				SessionID:  s.Config.ID,
				AgentID:    "orchestrator",
				Role:       "orchestrator",
				Action:     "Sesión finalizada exitosamente",
				Stage:      logger.StageFinish,
				Status:     &st,
				DurationMs: &dur,
			})
		}
	}

	return runErr
}
