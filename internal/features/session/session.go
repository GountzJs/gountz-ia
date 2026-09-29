package session

import (
	"context"
	"errors"
	"fmt"
	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/metrics"
	"gz-ia/internal/features/tooling"
	"gz-ia/internal/features/vault"
	"gz-ia/internal/features/workspace"
	"os"
	"os/exec"
	"reflect"
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

// DefaultOrchestratorPrompt define el prompt inicial por defecto para iniciar de forma proactiva
// el rol de Agente Orquestador cuando el usuario no suministra un prompt inicial.
const DefaultOrchestratorPrompt = `Eres el Agente Orquestador de gz-ia para esta sesión.
Tu función es exclusivamente de Gobernanza, Planificación y Dirección Técnica.
REGLAS OPERATIVAS OBLIGATORIAS:
1. Prohibido codificar directamente, realizar lecturas masivas o editar archivos en este hilo principal: DEBES delegar sistemáticamente a subagentes especializados (invoke_subagent).
2. Prohibido formular cuestionarios pasivos al usuario (ask_question): analiza proactivamente con subagentes de investigación y propón planes concretos y ejecutables.
3. Saluda al usuario, reporta brevemente el estado del workspace, herramientas MCP detectadas y directivas activas, y queda a disposición para coordinar el trabajo.`

// Config contiene los parámetros para iniciar una sesión de chat con el agente.
type Config struct {
	ID              string
	Provider        string
	WorkingDir      string
	InitialPrompt   string
	PermissionLevel PermissionLevel
	Resume          bool
	ReloadToolkits  bool
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
		activeToolkitIDs, err := s.Tooling.ResolveToolkits(ctx, s.Config.Profiles)
		if err != nil {
			return fmt.Errorf("error al resolver toolkits en tooling: %w", err)
		}

		composedTooling, err := s.Tooling.ComposeToolkits(ctx, activeToolkitIDs)
		if err != nil {
			return fmt.Errorf("error al componer toolkits en tooling: %w", err)
		}
		if composedTooling != nil {
			// Integrar variables de entorno y servidores MCP de los presets si aplica
			for _, profName := range s.Config.Profiles {
				if preset, err := s.Tooling.GetPreset(ctx, profName); err == nil && preset != nil {
					for k, v := range preset.MCPServers {
						if k == "gz-ia" {
							return fmt.Errorf("el servidor MCP 'gz-ia' está reservado para uso interno y no puede ser definido por un perfil")
						}
						if existing, conflict := composedTooling.MCPServers[k]; conflict {
							if !reflect.DeepEqual(existing, v) {
								return fmt.Errorf("colisión de servidores MCP: el servidor '%s' está definido con configuraciones distintas", k)
							}
						}
						composedTooling.MCPServers[k] = v
					}
					for k, v := range preset.Env {
						if existingVal, conflict := composedTooling.Env[k]; conflict && existingVal != v {
							return fmt.Errorf("colisión de variables de entorno: variable '%s' con valores distintos (%s vs %s)", k, existingVal, v)
						}
						composedTooling.Env[k] = v
					}
				}
			}

			// Si es reanudación y ya existe un manifiesto previo, no volver a reproyectar desde cero (salvo que ReloadToolkits sea true)
			shouldProject := true
			if s.Config.Resume {
				if existingManifest, err := workspace.LoadManifest(ws.WorkingDir, s.Config.ID); err == nil && existingManifest != nil {
					shouldProject = false
				}
			}

			if s.Config.ReloadToolkits {
				shouldProject = true
			}

			if shouldProject {
				if projErr := s.Tooling.ProjectIntoWorktree(ctx, ws.TargetDir, composedTooling, s.Config.ID, ws.WorkingDir); projErr != nil {
					return fmt.Errorf("error al proyectar tooling en el worktree: %w", projErr)
				}
			}
		}
	}

	// Si la sesión no es aislada (sin Git worktree), garantizar desproyección limpia al finalizar
	if !ws.IsIsolated && s.Tooling != nil {
		defer func() {
			if manifest, err := workspace.LoadManifest(ws.WorkingDir, s.Config.ID); err == nil && manifest != nil {
				_ = s.Tooling.Unproject(context.Background(), ws.TargetDir, manifest)
			}
		}()
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

		initialPrompt := s.Config.InitialPrompt
		if len(initialPrompt) > 200 {
			initialPrompt = initialPrompt[:200] + "..."
		}

		_ = s.Logger.Emit(ctx, &logger.Event{
			SessionID: s.Config.ID,
			AgentID:   "orchestrator",
			Role:      "orchestrator",
			Action:    action,
			Stage:     logger.StagePending,
			Status:    nil,
			Metadata: map[string]any{
				"provider":       s.Config.Provider,
				"toolkits":       s.Config.Profiles,
				"branch":         "harness/" + s.Config.ID,
				"initial_prompt": initialPrompt,
				"working_dir":    s.Config.WorkingDir,
			},
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

	var runErr error
	var exitCode int

	defer func() {
		if r := recover(); r != nil {
			if runErr == nil {
				runErr = fmt.Errorf("panic durante la ejecución de la sesión: %v", r)
			}
		}

		// Comprobar si fue marcada como killed externamente
		isKilled := false
		if latest, getErr := s.Store.Get(s.Config.ID); getErr == nil && latest != nil {
			if latest.Status == StatusKilled {
				isKilled = true
			}
		}

		if !isKilled {
			endTime := time.Now()
			record.FinishedAt = &endTime
			record.DurationMs = endTime.Sub(startTime).Milliseconds()
			record.ExitCode = exitCode

			if runErr != nil {
				record.Status = StatusFailed
			} else {
				record.Status = StatusCompleted
			}
		}

		// Fase 2: Buscar ConversationID y sincronizar telemetría a events.jsonl
		_ = s.SyncTelemetry(ctx, record)

		// Fase 3: Actualizar manifiesto dinámico delta
		if ws != nil && ws.TargetDir != "" {
			manifestPath := workspace.ManifestPath(record.WorkingDir, record.ID)
			_ = workspace.UpdateManifestDelta(manifestPath, ws.TargetDir)
		}

		_ = s.Store.Save(record)

		if s.Logger != nil && !isKilled {
			dur := record.DurationMs
			finishMeta := map[string]any{
				"exit_code":  exitCode,
				"duration_s": record.DurationMs / 1000,
			}
			if s.Workspace != nil {
				if diff, diffErr := s.Workspace.DiffWorktree(ctx, record.WorkingDir, record.WorktreeDir, record.BranchName, true); diffErr == nil && diff != "" {
					finishMeta["files_changed"] = diff
				}
			}
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
					Metadata:   finishMeta,
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
					Metadata:   finishMeta,
				})
			}
		}
	}()

	exitCode, runErr = s.Runner.Run(ctx, binary, args, ws.TargetDir, onStart)
	return runErr
}

// SyncTelemetry analiza el transcript de Antigravity (usando metrics.Collector) y registra tool calls
// y subagentes invocados como eventos en events.jsonl.
func (s *Session) SyncTelemetry(ctx context.Context, record *SessionRecord) error {
	if record == nil {
		return nil
	}

	targetDirs := []string{record.WorkingDir}
	if record.WorktreeDir != "" {
		targetDirs = append(targetDirs, record.WorktreeDir)
	}

	collector := metrics.NewCollector()
	convID := record.ConversationID
	if convID == "" {
		foundID, err := collector.FindConversationID(record.ID, targetDirs, record.StartedAt)
		if err == nil && foundID != "" {
			convID = foundID
			record.ConversationID = convID
		}
	}

	if convID == "" {
		return nil
	}

	tPath := collector.ResolveTranscriptPath(convID)
	steps, err := collector.ParseTranscriptSteps(tPath)
	if err != nil || len(steps) == 0 {
		return nil
	}

	if s.Logger == nil {
		return nil
	}

	existingEvents, _ := s.Logger.GetEvents(ctx, record.ID)
	seen := make(map[string]bool)
	for _, evt := range existingEvents {
		if evt.Metadata != nil {
			if key, ok := evt.Metadata["event_key"].(string); ok && key != "" {
				seen[key] = true
			}
		}
	}

	for _, step := range steps {
		stepTime := time.Now().UTC()
		if t, err := time.Parse(time.RFC3339Nano, step.CreatedAt); err == nil {
			stepTime = t
		} else if t, err := time.Parse(time.RFC3339, step.CreatedAt); err == nil {
			stepTime = t
		}

		for tcIdx, tc := range step.ToolCalls {
			eventKey := fmt.Sprintf("step-%d-tc-%d-%s", step.StepIndex, tcIdx, tc.Name)
			if seen[eventKey] {
				continue
			}

			stage := logger.StagePending
			if isReadTool(tc.Name) {
				stage = logger.StageRead
			}

			var status *logger.Status
			if step.Status == "ERROR" {
				st := logger.StatusFailed
				status = &st
			} else {
				st := logger.StatusOK
				status = &st
			}

			meta := map[string]any{
				"event_key":  eventKey,
				"tool":       tc.Name,
				"step_index": step.StepIndex,
				"source":     step.Source,
			}
			if len(tc.Args) > 0 {
				meta["args"] = string(tc.Args)
			}

			action := fmt.Sprintf("tool_call:%s", tc.Name)

			_ = s.Logger.Emit(ctx, &logger.Event{
				SessionID: record.ID,
				AgentID:   "orchestrator",
				Role:      "orchestrator",
				Action:    action,
				Stage:     stage,
				Status:    status,
				Timestamp: stepTime,
				Metadata:  meta,
			})
			seen[eventKey] = true

			if tc.Name == "invoke_subagent" {
				subArgs, _ := metrics.ParseSubagentArgs(tc.Args)
				for subIdx, subArg := range subArgs {
					subKey := fmt.Sprintf("step-%d-tc-%d-sub-%d", step.StepIndex, tcIdx, subIdx)
					if seen[subKey] {
						continue
					}
					subMeta := map[string]any{
						"event_key":     subKey,
						"subagent_role": subArg.Role,
						"subagent_type": subArg.TypeName,
						"prompt":        subArg.Prompt,
						"model":         subArg.Model,
						"workspace":     subArg.Workspace,
					}
					st := logger.StatusOK
					_ = s.Logger.Emit(ctx, &logger.Event{
						SessionID: record.ID,
						AgentID:   subArg.TypeName,
						Role:      subArg.Role,
						Action:    fmt.Sprintf("invoke_subagent:%s", subArg.Role),
						Stage:     logger.StagePending,
						Status:    &st,
						Timestamp: stepTime,
						Metadata:  subMeta,
					})
					seen[subKey] = true
				}
			}
		}
	}

	return nil
}

func isReadTool(name string) bool {
	switch name {
	case "view_file", "search_web", "read_url_content", "read_browser_page", "manage_task", "schedule", "ask_question":
		return true
	default:
		return false
	}
}

