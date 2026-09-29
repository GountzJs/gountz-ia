package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/metrics"
	"gz-ia/internal/features/tooling"
	"gz-ia/internal/features/vault"
	"gz-ia/internal/features/workspace"
)

// StartChatRequest encapsula los parámetros necesarios para iniciar una sesión de chat agéntica.
type StartChatRequest struct {
	ID              string
	Provider        string
	WorkingDir      string
	InitialPrompt   string
	PermissionLevel PermissionLevel
	BinaryPath      string
	Profiles        []string
	OnLaunch        func(id string, isIsolated bool)
}

// MergeOptions define los parámetros para integrar cambios de un worktree aislado al repo principal.
type MergeOptions struct {
	Squash   bool
	NoCommit bool
}

// PruneResult contiene las entidades huérfanas eliminadas tras la reconciliación.
type PruneResult struct {
	PrunedWorktrees []string `json:"pruned_worktrees"`
	DeletedBranches []string `json:"deleted_branches"`
}

// Service define el contrato de casos de uso agnósticos de la UI para la gestión de sesiones.
type Service interface {
	StartChat(ctx context.Context, req StartChatRequest) error
	List(ctx context.Context) ([]SessionRecord, error)
	GetRecord(ctx context.Context, id string) (*SessionRecord, error)
	GetSession(ctx context.Context, id string) (*SessionRecord, error)
	Get(ctx context.Context, id string) (*workspace.MergeResult, error)
	Read(ctx context.Context, id string, statOnly bool) (string, error)
	Kill(ctx context.Context, id string) error
	Resume(ctx context.Context, id string, reloadToolkits ...bool) error
	ReloadToolkits(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	Path(ctx context.Context, id string) (string, error)
	Diff(ctx context.Context, id string, statOnly bool) (string, error)
	Merge(ctx context.Context, id string, opts MergeOptions) (*workspace.MergeResult, error)
	Metrics(ctx context.Context, id string) (*metrics.SessionMetrics, error)
	LogEvent(ctx context.Context, evt *logger.Event) error
	Context(ctx context.Context, id string) (*SessionContext, error)
	GetEvents(ctx context.Context, id string) ([]logger.Event, error)
	WatchEvents(ctx context.Context, id string) (<-chan logger.Event, error)
	Cleanup(ctx context.Context, id string) error
	Prune(ctx context.Context) (*PruneResult, error)
	Vault() vault.Service
}

// sessionService implementa la interfaz Service orquestando los componentes centrales.
type sessionService struct {
	workDir   string
	store     Store
	runner    Runner
	killer    ProcessKiller
	workspace workspace.Provider
	metrics   metrics.Service
	logger    logger.Service
	vault     vault.Service
	tooling   tooling.Service
}

// Option permite configurar dependencias opcionales en NewService.
type Option func(*sessionService)

// WithTooling inyecta un servicio de tooling modular personalizado.
func WithTooling(t tooling.Service) Option {
	return func(svc *sessionService) {
		svc.tooling = t
	}
}

// WithVault inyecta un servicio de vault personalizado.
func WithVault(v vault.Service) Option {
	return func(svc *sessionService) {
		svc.vault = v
	}
}

// WithStore inyecta un almacén de sesiones personalizado.
func WithStore(s Store) Option {
	return func(svc *sessionService) {
		svc.store = s
	}
}

// WithRunner inyecta un ejecutor de procesos personalizado.
func WithRunner(r Runner) Option {
	return func(svc *sessionService) {
		svc.runner = r
	}
}

// WithKiller inyecta un terminador de procesos personalizado.
func WithKiller(k ProcessKiller) Option {
	return func(svc *sessionService) {
		svc.killer = k
	}
}

// WithWorkspace inyecta un proveedor de espacios de trabajo personalizado.
func WithWorkspace(ws workspace.Provider) Option {
	return func(svc *sessionService) {
		svc.workspace = ws
	}
}

// WithMetrics inyecta un servicio de métricas personalizado.
func WithMetrics(m metrics.Service) Option {
	return func(svc *sessionService) {
		svc.metrics = m
	}
}

// WithLogger inyecta un servicio de observabilidad y registro de eventos.
func WithLogger(l logger.Service) Option {
	return func(svc *sessionService) {
		svc.logger = l
	}
}

// NewService crea un nuevo servicio de sesión con dependencias inyectadas o valores por defecto limpios.
func NewService(workDir string, opts ...Option) Service {
	svc := &sessionService{
		workDir: workDir,
	}

	for _, opt := range opts {
		opt(svc)
	}

	if svc.workspace == nil {
		svc.workspace = workspace.NewDefaultProvider()
	}

	if svc.workDir == "" {
		cwd, _ := os.Getwd()
		svc.workDir = cwd
	}
	svc.workDir = svc.workspace.ResolveProjectRoot(context.Background(), svc.workDir)

	if svc.store == nil {
		svc.store = DefaultFileStore(svc.workDir)
	}

	if svc.runner == nil {
		svc.runner = &OSRunner{}
	}

	if svc.killer == nil {
		svc.killer = &OSProcessKiller{}
	}

	if svc.metrics == nil {
		svc.metrics = metrics.NewService()
	}

	if svc.logger == nil {
		svc.logger = logger.NewService(svc.workDir)
	}

	if svc.vault == nil {
		svc.vault = vault.NewService(svc.workDir)
	}

	if svc.tooling == nil {
		svc.tooling = tooling.NewService("", svc.workDir)
	}

	return svc
}

func (s *sessionService) Vault() vault.Service {
	return s.vault
}

func (s *sessionService) StartChat(ctx context.Context, req StartChatRequest) error {
	targetDir := req.WorkingDir
	if targetDir == "" {
		targetDir = s.workDir
	}
	if s.workspace != nil {
		targetDir = s.workspace.ResolveProjectRoot(ctx, targetDir)
	}

	id := req.ID
	if id == "" {
		id = GenerateID()
	}

	prov := req.Provider
	if prov == "" {
		if first := FirstAvailableDriver(); first != nil {
			prov = first.ID()
		} else {
			prov = "agy"
		}
	}

	bin := req.BinaryPath
	if bin == "" {
		if d, err := GetDriver(prov); err == nil {
			bin = d.BinaryName()
		} else {
			bin = "agy"
		}
	}

	perm := req.PermissionLevel
	if perm == "" {
		perm = PermissionSupervised
	}

	// Validar variables de entorno requeridas y advertir al usuario si no existen
	if s.vault != nil {
		var requiredKeys []string
		if s.tooling != nil && len(req.Profiles) > 0 {
			if tkIDs, err := s.tooling.ResolveToolkits(ctx, req.Profiles); err == nil {
				if composed, err := s.tooling.ComposeToolkits(ctx, tkIDs); err == nil && composed != nil {
					for k := range composed.Env {
						requiredKeys = append(requiredKeys, k)
					}
				}
			}
			for _, pName := range req.Profiles {
				if preset, err := s.tooling.GetPreset(ctx, pName); err == nil && preset != nil {
					for k := range preset.Env {
						requiredKeys = append(requiredKeys, k)
					}
				}
			}
		}

		if len(requiredKeys) > 0 {
			if missing, err := s.vault.ValidateRequired(ctx, requiredKeys); err == nil && len(missing) > 0 {
				fmt.Fprintf(os.Stderr, "\n\033[1;33m▲ Aviso de Entorno:\033[0m Se detectaron variables no configuradas en el entorno ni en el vault:\n")
				for _, k := range missing {
					fmt.Fprintf(os.Stderr, "   • \033[1m%s\033[0m\n", k)
				}
				fmt.Fprintf(os.Stderr, "   Puedes configurarlas en el vault seguro con: \033[36mgz-ia vault set <VARIABLE>\033[0m\n\n")

				if s.logger != nil {
					_ = s.logger.Emit(ctx, &logger.Event{
						SessionID: id,
						AgentID:   "orchestrator",
						Role:      "orchestrator",
						Action:    fmt.Sprintf("Aviso: variables de entorno no encontradas (%s)", strings.Join(missing, ", ")),
						Stage:     logger.StagePending,
					})
				}
			}
		}
	}

	if req.OnLaunch != nil {
		isIsolated := false
		if s.workspace != nil {
			isIsolated = s.workspace.IsGitAvailable(ctx, targetDir)
		}
		req.OnLaunch(id, isIsolated)
	}

	initPrompt := req.InitialPrompt
	if initPrompt == "" {
		initPrompt = DefaultOrchestratorPrompt
	}

	cfg := Config{
		ID:              id,
		Provider:        prov,
		WorkingDir:      targetDir,
		InitialPrompt:   initPrompt,
		PermissionLevel: perm,
		BinaryPath:      bin,
		Profiles:        req.Profiles,
	}

	sess := New(cfg, s.runner, s.store).
		WithWorkspace(s.workspace).
		WithLogger(s.logger).
		WithVault(s.vault).
		WithTooling(s.tooling)
	return sess.Start(ctx)
}

func (s *sessionService) List(ctx context.Context) ([]SessionRecord, error) {
	if s.store == nil {
		return nil, errors.New("store no inicializado")
	}
	return s.store.List()
}

func (s *sessionService) GetRecord(ctx context.Context, id string) (*SessionRecord, error) {
	if s.store == nil {
		return nil, errors.New("store no inicializado")
	}
	return s.store.Get(id)
}

func (s *sessionService) GetSession(ctx context.Context, id string) (*SessionRecord, error) {
	return s.GetRecord(ctx, id)
}

func (s *sessionService) Kill(ctx context.Context, id string) error {
	if s.store == nil {
		return errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return err
	}

	if record.Status != StatusRunning {
		return fmt.Errorf("la sesión '%s' no está activa (estado actual: %s)", id, record.Status)
	}

	if record.PID > 0 {
		if err := s.killer.Kill(record.PID); err != nil {
			return fmt.Errorf("error al terminar proceso %d de la sesión '%s': %w", record.PID, id, err)
		}
	}

	now := time.Now()
	record.Status = StatusKilled
	record.FinishedAt = &now
	record.DurationMs = now.Sub(record.StartedAt).Milliseconds()

	if err := s.store.Save(record); err != nil {
		return fmt.Errorf("error al actualizar estado de la sesión '%s': %w", id, err)
	}

	if s.logger != nil {
		dur := record.DurationMs
		st := logger.StatusFailed
		_ = s.logger.Emit(ctx, &logger.Event{
			SessionID:  id,
			AgentID:    "orchestrator",
			Role:       "orchestrator",
			Action:     fmt.Sprintf("Sesión finalizada manualmente (kill PID %d)", record.PID),
			Stage:      logger.StageFinish,
			Status:     &st,
			DurationMs: &dur,
		})
	}

	return nil
}

func (s *sessionService) Resume(ctx context.Context, id string, reloadToolkits ...bool) error {
	shouldReload := false
	if len(reloadToolkits) > 0 {
		shouldReload = reloadToolkits[0]
	}

	if s.store == nil {
		return errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return err
	}

	if record.Status == StatusRunning {
		return fmt.Errorf("la sesión '%s' ya se encuentra en ejecución (PID %d)", id, record.PID)
	}

	prov := record.Provider
	if prov == "" {
		prov = "agy"
	}
	bin := prov
	if d, err := GetDriver(prov); err == nil {
		bin = d.BinaryName()
	}

	cfg := Config{
		ID:              record.ID,
		Provider:        prov,
		WorkingDir:      record.WorkingDir,
		PermissionLevel: record.PermissionLevel,
		Resume:          true,
		ReloadToolkits:  shouldReload,
		BinaryPath:      bin,
		IsIsolated:      record.IsIsolated,
		WorktreeDir:     record.WorktreeDir,
		BranchName:      record.BranchName,
		Profiles:        record.Profiles,
	}

	sess := New(cfg, s.runner, s.store).
		WithWorkspace(s.workspace).
		WithLogger(s.logger).
		WithVault(s.vault).
		WithTooling(s.tooling)
	return sess.Start(ctx)
}

func (s *sessionService) ReloadToolkits(ctx context.Context, id string) error {
	if s.store == nil {
		return errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return err
	}

	targetDir := record.WorkingDir
	if record.IsIsolated && record.WorktreeDir != "" {
		targetDir = record.WorktreeDir
	}

	if s.tooling == nil {
		return errors.New("servicio de tooling no disponible")
	}

	if len(record.Profiles) == 0 {
		return fmt.Errorf("la sesión '%s' no tiene toolkits ni perfiles asociados para recargar", id)
	}

	if err := s.tooling.ReloadToolkits(ctx, targetDir, record.Profiles, record.ID, s.workDir); err != nil {
		return fmt.Errorf("error al recargar toolkits para la sesión '%s': %w", id, err)
	}

	if s.logger != nil {
		_ = s.logger.Emit(ctx, &logger.Event{
			SessionID: id,
			AgentID:   "orchestrator",
			Role:      "orchestrator",
			Action:    fmt.Sprintf("Toolkits recargados exitosamente (%s)", strings.Join(record.Profiles, ", ")),
			Stage:     logger.StagePending,
		})
	}

	return nil
}

func (s *sessionService) Delete(ctx context.Context, id string) error {
	if s.store == nil {
		return errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return err
	}

	if record.Status == StatusRunning && record.PID > 0 {
		_ = s.killer.Kill(record.PID)
	}

	if record.WorktreeDir != "" {
		ws := s.workspace
		if ws == nil {
			ws = workspace.NewDefaultProvider()
		}
		_ = ws.CleanupWorktree(ctx, record.WorkingDir, record.WorktreeDir, record.BranchName)
	}

	return s.store.Delete(id)
}

func (s *sessionService) Path(ctx context.Context, id string) (string, error) {
	if s.store == nil {
		return "", errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return "", err
	}

	targetPath := record.WorkingDir
	if record.IsIsolated && record.WorktreeDir != "" {
		targetPath = record.WorktreeDir
	}

	absPath, err := filepath.Abs(targetPath)
	if err == nil {
		targetPath = absPath
	}
	return filepath.Clean(targetPath), nil
}

func (s *sessionService) Diff(ctx context.Context, id string, statOnly bool) (string, error) {
	if s.store == nil {
		return "", errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return "", err
	}

	if !record.IsIsolated {
		return "La sesión se ejecutó en modo directo y no cuenta con diff aislado.", nil
	}

	ws := s.workspace
	if ws == nil {
		ws = workspace.NewDefaultProvider()
	}

	return ws.DiffWorktree(ctx, record.WorkingDir, record.WorktreeDir, record.BranchName, statOnly)
}

func (s *sessionService) Merge(ctx context.Context, id string, opts MergeOptions) (*workspace.MergeResult, error) {
	if s.store == nil {
		return nil, errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}

	if !record.IsIsolated {
		return nil, fmt.Errorf("la sesión '%s' se ejecutó en modo directo y no cuenta con una rama de worktree aislada para fusionar", id)
	}

	branchName := record.BranchName
	if branchName == "" {
		branchName = "harness/" + record.ID
	}

	ws := s.workspace
	if ws == nil {
		ws = workspace.NewDefaultProvider()
	}

	return ws.MergeWorktree(ctx, record.ID, record.WorkingDir, record.WorktreeDir, branchName, opts.Squash, opts.NoCommit)
}

func (s *sessionService) Read(ctx context.Context, id string, statOnly bool) (string, error) {
	return s.Diff(ctx, id, statOnly)
}

func (s *sessionService) Get(ctx context.Context, id string) (*workspace.MergeResult, error) {
	if s.store == nil {
		return nil, errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}

	if !record.IsIsolated {
		return nil, fmt.Errorf("la sesión '%s' se ejecutó en modo directo y no cuenta con una rama de worktree aislada para traer cambios", id)
	}

	branchName := record.BranchName
	if branchName == "" {
		branchName = "harness/" + record.ID
	}

	ws := s.workspace
	if ws == nil {
		ws = workspace.NewDefaultProvider()
	}

	return ws.GetWorktree(ctx, record.ID, record.WorkingDir, record.WorktreeDir, branchName)
}

func (s *sessionService) Metrics(ctx context.Context, id string) (*metrics.SessionMetrics, error) {
	if s.store == nil {
		return nil, errors.New("store no inicializado")
	}

	if s.metrics == nil {
		s.metrics = metrics.NewService()
	}

	record, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}

	targetDir := record.WorkingDir
	if record.IsIsolated && record.WorktreeDir != "" {
		targetDir = record.WorktreeDir
	}

	return s.metrics.GetMetrics(ctx, id, targetDir)
}

func (s *sessionService) Context(ctx context.Context, id string) (*SessionContext, error) {
	if s.store == nil {
		return nil, errors.New("store no inicializado")
	}
	if s.logger == nil {
		return nil, errors.New("logger no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}

	events, err := s.logger.GetEvents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("error al obtener eventos de la sesión '%s': %w", id, err)
	}

	sc := &SessionContext{
		SessionID:  record.ID,
		Provider:   record.Provider,
		Branch:     "harness/" + record.ID,
		Toolkits:   record.Profiles,
		WorkingDir: record.WorkingDir,
		Status:     string(record.Status),
		StartedAt:  record.StartedAt,
		FinishedAt: record.FinishedAt,
		DurationMs: record.DurationMs,
		Events:     events,
	}

	if sc.Toolkits == nil {
		sc.Toolkits = []string{}
	}

	for _, evt := range events {
		if evt.Stage == logger.StagePending && evt.Metadata != nil {
			if v, ok := evt.Metadata["provider"].(string); ok && v != "" {
				sc.Provider = v
			}
			if v, ok := evt.Metadata["toolkits"].([]string); ok {
				sc.Toolkits = v
			} else if v, ok := evt.Metadata["toolkits"].([]any); ok {
				toolkits := make([]string, 0, len(v))
				for _, t := range v {
					if s, ok := t.(string); ok {
						toolkits = append(toolkits, s)
					}
				}
				sc.Toolkits = toolkits
			}
			if v, ok := evt.Metadata["branch"].(string); ok && v != "" {
				sc.Branch = v
			}
			if v, ok := evt.Metadata["initial_prompt"].(string); ok {
				sc.InitialPrompt = v
			}
			if v, ok := evt.Metadata["working_dir"].(string); ok && v != "" {
				sc.WorkingDir = v
			}
			break
		}
	}

	for i := len(events) - 1; i >= 0; i-- {
		evt := events[i]
		if evt.Stage == logger.StageFinish && evt.Metadata != nil {
			if v, ok := evt.Metadata["files_changed"].(string); ok && v != "" {
				sc.FilesChanged = v
			}
			break
		}
	}

	return sc, nil
}

func (s *sessionService) LogEvent(ctx context.Context, evt *logger.Event) error {
	if s.logger == nil {
		return errors.New("logger no inicializado")
	}
	return s.logger.Emit(ctx, evt)
}

func (s *sessionService) GetEvents(ctx context.Context, id string) ([]logger.Event, error) {
	if s.logger == nil {
		return nil, errors.New("logger no inicializado")
	}
	return s.logger.GetEvents(ctx, id)
}

func (s *sessionService) WatchEvents(ctx context.Context, id string) (<-chan logger.Event, error) {
	if s.logger == nil {
		return nil, errors.New("logger no inicializado")
	}
	return s.logger.Watch(ctx, id)
}

func (s *sessionService) Cleanup(ctx context.Context, id string) error {
	if s.store == nil {
		return errors.New("store no inicializado")
	}

	record, err := s.store.Get(id)
	if err != nil {
		return err
	}

	targetDir := record.WorkingDir
	if record.IsIsolated && record.WorktreeDir != "" {
		targetDir = record.WorktreeDir
	}

	if s.tooling != nil {
		if manifest, err := workspace.LoadManifest(s.workDir, id); err == nil && manifest != nil {
			if err := s.tooling.Unproject(ctx, targetDir, manifest); err != nil {
				return fmt.Errorf("error al desproyectar sesión '%s': %w", id, err)
			}
		}
	}

	return nil
}

func (s *sessionService) Prune(ctx context.Context) (*PruneResult, error) {
	records, err := s.store.List()
	if err != nil {
		return nil, fmt.Errorf("error al listar sesiones para reconciliación: %w", err)
	}

	var activeIDs []string
	for _, rec := range records {
		activeIDs = append(activeIDs, rec.ID)
	}

	report, err := s.workspace.Prune(ctx, s.workDir, activeIDs)
	if err != nil {
		return nil, fmt.Errorf("error al podar worktrees y ramas de Git: %w", err)
	}

	// Limpiar proyecciones huérfanas en sesiones terminadas no aisladas
	if s.tooling != nil {
		for _, rec := range records {
			if !rec.IsIsolated && (rec.Status == StatusCompleted || rec.Status == StatusFailed || rec.Status == StatusKilled) {
				if manifest, err := workspace.LoadManifest(s.workDir, rec.ID); err == nil && manifest != nil {
					_ = s.tooling.Unproject(ctx, rec.WorkingDir, manifest)
				}
			}
		}
	}

	return &PruneResult{
		PrunedWorktrees: report.PrunedWorktrees,
		DeletedBranches: report.DeletedBranches,
	}, nil
}
