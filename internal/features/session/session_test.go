package session

import (
	"context"
	"errors"
	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/tooling"
	"gz-ia/internal/features/workspace"
	"os"
	"reflect"
	"strings"
	"testing"
)

type mockRunner struct {
	lastBinary string
	lastArgs   []string
	lastDir    string
	returnErr  error
	mockPID    int
}

func (m *mockRunner) Run(ctx context.Context, binary string, args []string, dir string, onStart func(pid int)) (int, error) {
	m.lastBinary = binary
	m.lastArgs = args
	m.lastDir = dir
	pid := m.mockPID
	if pid == 0 {
		pid = 1234
	}
	if onStart != nil {
		onStart(pid)
	}
	exitCode := 0
	if m.returnErr != nil {
		exitCode = 1
	}
	return exitCode, m.returnErr
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BinaryPath != "agy" {
		t.Errorf("BinaryPath esperado 'agy', obtenido '%s'", cfg.BinaryPath)
	}
	if cfg.WorkingDir == "" {
		t.Error("WorkingDir no debe estar vacío")
	}
	if cfg.PermissionLevel != PermissionSupervised {
		t.Errorf("PermissionLevel esperado '%s', obtenido '%s'", PermissionSupervised, cfg.PermissionLevel)
	}
	if cfg.InitialPrompt != "" {
		t.Errorf("InitialPrompt esperado vacío, obtenido '%s'", cfg.InitialPrompt)
	}
	if cfg.Provider != "agy" {
		t.Errorf("Provider esperado 'agy', obtenido '%s'", cfg.Provider)
	}
	if len(cfg.ID) != 8 {
		t.Errorf("ID esperado de 8 caracteres, obtenido '%s'", cfg.ID)
	}
}

func TestResolveBinaryPath(t *testing.T) {
	path, err := ResolveBinaryPath("git")
	if err != nil {
		t.Fatalf("ResolveBinaryPath('git') falló: %v", err)
	}
	if !strings.Contains(path, "git") {
		t.Errorf("ruta resuelta inesperada: %s", path)
	}

	_, err = ResolveBinaryPath("binario_inexistente_12345")
	if err == nil {
		t.Fatal("se esperaba error con binario inexistente y se obtuvo nil")
	}
}

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name       string
		cfg        Config
		wantArgs   []string
		wantErr    bool
		errContain string
	}{
		{
			name: "Nivel Supervised por defecto (sin prompt)",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: PermissionSupervised,
			},
			wantArgs: nil,
			wantErr:  false,
		},
		{
			name: "Nivel ReadOnly (solo lectura / modo plan)",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: PermissionReadOnly,
			},
			wantArgs: []string{"--mode", "plan"},
			wantErr:  false,
		},
		{
			name: "Nivel Autonomous (auto-aprobación en workspace)",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: PermissionAutonomous,
			},
			wantArgs: []string{"--dangerously-skip-permissions"},
			wantErr:  false,
		},
		{
			name: "Nivel ReadOnly con InitialPrompt",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: PermissionReadOnly,
				InitialPrompt:   "auditar seguridad",
			},
			wantArgs: []string{"--mode", "plan", "-i", "auditar seguridad"},
			wantErr:  false,
		},
		{
			name: "Nivel Autonomous con InitialPrompt",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: PermissionAutonomous,
				InitialPrompt:   "generar componente",
			},
			wantArgs: []string{"--dangerously-skip-permissions", "-i", "generar componente"},
			wantErr:  false,
		},
		{
			name: "Reanudación de sesión con Resume: true",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: PermissionSupervised,
				Resume:          true,
			},
			wantArgs: []string{"--continue"},
			wantErr:  false,
		},
		{
			name: "Error cuando WorkingDir está vacío",
			cfg: Config{
				WorkingDir: "",
			},
			wantErr:    true,
			errContain: "working directory no puede estar vacío",
		},
		{
			name: "Error con nivel de permiso desconocido",
			cfg: Config{
				WorkingDir:      "/test/dir",
				PermissionLevel: "invalido",
			},
			wantErr:    true,
			errContain: "nivel de permiso desconocido",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildArgs(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("se esperaba error y se obtuvo nil")
				}
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error inesperado: '%s' no contiene '%s'", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}

			if len(got) == 0 && len(tt.wantArgs) == 0 {
				return
			}

			if !reflect.DeepEqual(got, tt.wantArgs) {
				t.Errorf("BuildArgs() = %v, esperado %v", got, tt.wantArgs)
			}
		})
	}
}

func TestSessionStart_Success(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "session-start-test-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	mock := &mockRunner{mockPID: 4321}
	cfg := Config{
		ID:              "test0001",
		WorkingDir:      tmpDir,
		PermissionLevel: PermissionAutonomous,
		InitialPrompt:   "Hola agente",
		BinaryPath:      "agy",
	}

	sess := New(cfg, mock, store)
	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() retornó error inesperado: %v", err)
	}

	if mock.lastBinary != "agy" {
		t.Errorf("binario ejecutado '%s', esperado 'agy'", mock.lastBinary)
	}
	if mock.lastDir != tmpDir {
		t.Errorf("directorio ejecutado '%s', esperado '%s'", mock.lastDir, tmpDir)
	}

	expectedArgs := []string{"--dangerously-skip-permissions", "-i", "Hola agente"}
	if !reflect.DeepEqual(mock.lastArgs, expectedArgs) {
		t.Errorf("argumentos pasados %v, esperado %v", mock.lastArgs, expectedArgs)
	}

	// Verificar registro en Store
	rec, err := store.Get("test0001")
	if err != nil {
		t.Fatalf("Get('test0001') falló: %v", err)
	}
	if rec.Status != StatusCompleted {
		t.Errorf("estado esperado '%s', obtenido '%s'", StatusCompleted, rec.Status)
	}
	if rec.PID != 4321 {
		t.Errorf("PID esperado 4321, obtenido %d", rec.PID)
	}
	if rec.ExitCode != 0 {
		t.Errorf("ExitCode esperado 0, obtenido %d", rec.ExitCode)
	}
	if rec.FinishedAt == nil {
		t.Error("FinishedAt no debe ser nil")
	}
}

func TestSessionStart_RunnerError(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "session-err-test-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	expectedErr := errors.New("falla al spawnear proceso")
	mock := &mockRunner{returnErr: expectedErr}
	cfg := Config{
		ID:         "err0001",
		WorkingDir: tmpDir,
	}

	sess := New(cfg, mock, store)
	err := sess.Start(context.Background())
	if err == nil {
		t.Fatal("se esperaba error y se obtuvo nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error obtenido '%v', esperado '%v'", err, expectedErr)
	}

	rec, _ := store.Get("err0001")
	if rec.Status != StatusFailed {
		t.Errorf("estado esperado '%s', obtenido '%s'", StatusFailed, rec.Status)
	}
	if rec.ExitCode != 1 {
		t.Errorf("ExitCode esperado 1, obtenido %d", rec.ExitCode)
	}
}

func TestSessionStart_InvalidConfig(t *testing.T) {
	mock := &mockRunner{}
	cfg := Config{
		WorkingDir: "", // Inválido
	}

	sess := New(cfg, mock)
	err := sess.Start(context.Background())
	if err == nil {
		t.Fatal("se esperaba error con WorkingDir vacío")
	}
}

type mockWorkspaceProvider struct {
	isGitAvail bool
	prepareWS  *workspace.Workspace
	prepareErr error
	cleanedWS  *workspace.Workspace
	cleanErr   error
}

func (m *mockWorkspaceProvider) IsGitAvailable(ctx context.Context, dir string) bool {
	return m.isGitAvail
}

func (m *mockWorkspaceProvider) ResolveProjectRoot(ctx context.Context, dir string) string {
	return dir
}

func (m *mockWorkspaceProvider) Prepare(ctx context.Context, sessionID string, baseDir string) (*workspace.Workspace, error) {
	if m.prepareErr != nil {
		return nil, m.prepareErr
	}
	if m.prepareWS != nil {
		return m.prepareWS, nil
	}
	return &workspace.Workspace{
		WorkingDir:  baseDir,
		TargetDir:   baseDir,
		IsIsolated:  false,
		WorktreeDir: "",
		BranchName:  "",
	}, nil
}

func (m *mockWorkspaceProvider) Cleanup(ctx context.Context, ws *workspace.Workspace) error {
	m.cleanedWS = ws
	return m.cleanErr
}

func (m *mockWorkspaceProvider) CleanupWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string) error {
	m.cleanedWS = &workspace.Workspace{
		WorkingDir:  baseDir,
		WorktreeDir: worktreeDir,
		BranchName:  branchName,
		IsIsolated:  true,
	}
	return m.cleanErr
}

func (m *mockWorkspaceProvider) DiffWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string, statOnly bool) (string, error) {
	return "", nil
}

func (m *mockWorkspaceProvider) MergeWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string, squash bool, noCommit bool) (*workspace.MergeResult, error) {
	return &workspace.MergeResult{AlreadyUpToDate: true}, nil
}

func (m *mockWorkspaceProvider) GetWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string) (*workspace.MergeResult, error) {
	return &workspace.MergeResult{AlreadyUpToDate: true}, nil
}

func (m *mockWorkspaceProvider) Prune(ctx context.Context, baseDir string, activeSessionIDs []string) (*workspace.PruneReport, error) {
	return &workspace.PruneReport{}, nil
}

func TestSessionStart_WithGitWorktree(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "session-wt-test-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{mockPID: 6789}
	wtDir := tmpDir + "/.harness/worktrees/wt_sess_1"

	wsMock := &mockWorkspaceProvider{
		isGitAvail: true,
		prepareWS: &workspace.Workspace{
			WorkingDir:  tmpDir,
			TargetDir:   wtDir,
			IsIsolated:  true,
			WorktreeDir: wtDir,
			BranchName:  "harness/wt_sess_1",
		},
	}

	cfg := Config{
		ID:              "wt_sess_1",
		WorkingDir:      tmpDir,
		PermissionLevel: PermissionSupervised,
		BinaryPath:      "agy",
	}

	sess := New(cfg, mockRun, store).WithWorkspace(wsMock)
	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start con worktree falló: %v", err)
	}

	// Comprobar que el runner se ejecutó dentro del Worktree aislado
	if mockRun.lastDir != wtDir {
		t.Errorf("Runner debió ejecutarse en el WorktreeDir '%s', se ejecutó en '%s'", wtDir, mockRun.lastDir)
	}

	// Comprobar metadata en SessionRecord
	rec, err := store.Get("wt_sess_1")
	if err != nil {
		t.Fatalf("error obteniendo sesión: %v", err)
	}
	if !rec.IsIsolated {
		t.Error("rec.IsIsolated debió ser true")
	}
	if rec.WorktreeDir != wtDir {
		t.Errorf("rec.WorktreeDir esperado '%s', obtenido '%s'", wtDir, rec.WorktreeDir)
	}
	if rec.BranchName != "harness/wt_sess_1" {
		t.Errorf("rec.BranchName esperado 'harness/wt_sess_1', obtenido '%s'", rec.BranchName)
	}
	if rec.Status != StatusCompleted {
		t.Errorf("rec.Status esperado '%s', obtenido '%s'", StatusCompleted, rec.Status)
	}
}

func TestSessionStart_FallbackDirect(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "session-fallback-test-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{mockPID: 3333}

	wsMock := &mockWorkspaceProvider{
		isGitAvail: false,
		prepareWS: &workspace.Workspace{
			WorkingDir:  tmpDir,
			TargetDir:   tmpDir,
			IsIsolated:  false,
			WorktreeDir: "",
			BranchName:  "",
		},
	}

	cfg := Config{
		ID:              "fallback_sess",
		WorkingDir:      tmpDir,
		PermissionLevel: PermissionReadOnly,
	}

	sess := New(cfg, mockRun, store).WithWorkspace(wsMock)
	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start con fallback falló: %v", err)
	}

	if mockRun.lastDir != tmpDir {
		t.Errorf("Runner debió ejecutarse en tmpDir directo, obtenido: %s", mockRun.lastDir)
	}

	rec, _ := store.Get("fallback_sess")
	if rec.IsIsolated {
		t.Error("rec.IsIsolated debió ser false")
	}
	if rec.WorktreeDir != "" {
		t.Errorf("rec.WorktreeDir debió ser vacío, obtenido: %s", rec.WorktreeDir)
	}
}

func TestSessionStart_ResumeIsolated(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "session-resume-iso-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	wtDir := tmpDir + "/.harness/worktrees/resume_iso"

	wsMock := &mockWorkspaceProvider{isGitAvail: true}

	cfg := Config{
		ID:              "resume_iso",
		WorkingDir:      tmpDir,
		PermissionLevel: PermissionAutonomous,
		Resume:          true,
		IsIsolated:      true,
		WorktreeDir:     wtDir,
		BranchName:      "harness/resume_iso",
	}

	sess := New(cfg, mockRun, store).WithWorkspace(wsMock)
	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() con reanudación aislada falló: %v", err)
	}

	if mockRun.lastDir != wtDir {
		t.Errorf("Runner debió reutilizar el worktree existente '%s', ejecutado en '%s'", wtDir, mockRun.lastDir)
	}
}

func TestSessionStart_WorkspacePrepError(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "session-prep-err-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	wsMock := &mockWorkspaceProvider{
		prepareErr: errors.New("falla al inicializar worktree git"),
	}

	cfg := Config{
		ID:         "prep_err_sess",
		WorkingDir: tmpDir,
	}

	sess := New(cfg, mockRun, store).WithWorkspace(wsMock)
	err := sess.Start(context.Background())
	if err == nil {
		t.Fatal("Start debió fallar cuando Prepare del workspace falla")
	}
	if !strings.Contains(err.Error(), "error al preparar espacio de trabajo") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}

type mockToolingService struct {
	tooling.Service
	resolveToolkitsFunc func(ctx context.Context, names []string) ([]string, error)
	composeToolkitsFunc func(ctx context.Context, ids []string) (*tooling.ComposedTooling, error)
	getPresetFunc       func(ctx context.Context, name string) (*tooling.Preset, error)
	projectWorktreeFunc func(ctx context.Context, targetDir string, composed *tooling.ComposedTooling, sessionID string, baseDir ...string) error
	unprojectFunc       func(ctx context.Context, targetDir string, manifest *workspace.Manifest) error
	reloadToolkitsFunc  func(ctx context.Context, targetDir string, profiles []string, sessionID string, baseDir ...string) error
	projectCalled       bool
	unprojectCalled     bool
	reloadCalled        bool
}

func (m *mockToolingService) ResolveToolkits(ctx context.Context, names []string) ([]string, error) {
	if m.resolveToolkitsFunc != nil {
		return m.resolveToolkitsFunc(ctx, names)
	}
	return names, nil
}

func (m *mockToolingService) ComposeToolkits(ctx context.Context, ids []string) (*tooling.ComposedTooling, error) {
	if m.composeToolkitsFunc != nil {
		return m.composeToolkitsFunc(ctx, ids)
	}
	return &tooling.ComposedTooling{
		ActiveToolkits: ids,
		AgentsFiles:    make(map[string]string),
		RulesFiles:     make(map[string]string),
		SkillPaths:     make(map[string]string),
		Tools:          nil,
		MCPServers:     make(map[string]any),
		Env:            make(map[string]string),
	}, nil
}

func (m *mockToolingService) GetPreset(ctx context.Context, name string) (*tooling.Preset, error) {
	if m.getPresetFunc != nil {
		return m.getPresetFunc(ctx, name)
	}
	return nil, errors.New("preset not found")
}

func (m *mockToolingService) ProjectIntoWorktree(ctx context.Context, targetDir string, composed *tooling.ComposedTooling, sessionID string, baseDir ...string) error {
	m.projectCalled = true
	if m.projectWorktreeFunc != nil {
		return m.projectWorktreeFunc(ctx, targetDir, composed, sessionID, baseDir...)
	}
	return nil
}

func (m *mockToolingService) Unproject(ctx context.Context, targetDir string, manifest *workspace.Manifest) error {
	m.unprojectCalled = true
	if m.unprojectFunc != nil {
		return m.unprojectFunc(ctx, targetDir, manifest)
	}
	return nil
}

func (m *mockToolingService) ReloadToolkits(ctx context.Context, targetDir string, profiles []string, sessionID string, baseDir ...string) error {
	m.reloadCalled = true
	if m.reloadToolkitsFunc != nil {
		return m.reloadToolkitsFunc(ctx, targetDir, profiles, sessionID, baseDir...)
	}
	return nil
}

func TestSessionStart_ResumeSkipsProjectingWhenManifestExists(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	toolingMock := &mockToolingService{}

	sessID := "resume_manifest_skip"
	// Guardar un manifiesto previo
	manifest := workspace.NewManifest(sessID)
	if err := workspace.SaveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("SaveManifest falló: %v", err)
	}

	cfg := Config{
		ID:         sessID,
		WorkingDir: tmpDir,
		Resume:     true,
		Profiles:   []string{"base-profile"},
	}

	sess := New(cfg, mockRun, store).WithTooling(toolingMock)
	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() falló: %v", err)
	}

	if toolingMock.projectCalled {
		t.Error("ProjectIntoWorktree NO debió ser llamado al reanudar una sesión que ya tenía manifiesto")
	}
}

func TestSessionStart_ResumeForcesProjectingWhenReloadToolkitsActive(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	toolingMock := &mockToolingService{}

	sessID := "resume_manifest_reload"
	// Guardar un manifiesto previo
	manifest := workspace.NewManifest(sessID)
	if err := workspace.SaveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("SaveManifest falló: %v", err)
	}

	cfg := Config{
		ID:             sessID,
		WorkingDir:     tmpDir,
		Resume:         true,
		ReloadToolkits: true,
		Profiles:       []string{"base-profile"},
	}

	sess := New(cfg, mockRun, store).WithTooling(toolingMock)
	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() falló: %v", err)
	}

	if !toolingMock.projectCalled {
		t.Error("ProjectIntoWorktree DEBIÓ ser llamado al reanudar con ReloadToolkits=true aun teniendo manifiesto")
	}
}

func TestSessionStart_ToolkitPresetGzIaProtection(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	toolingMock := &mockToolingService{
		getPresetFunc: func(ctx context.Context, name string) (*tooling.Preset, error) {
			return &tooling.Preset{
				Name: name,
				MCPServers: map[string]any{
					"gz-ia": map[string]any{"command": "fake"},
				},
			}, nil
		},
	}

	cfg := Config{
		ID:         "prot_gzia_sess",
		WorkingDir: tmpDir,
		Profiles:   []string{"hack-profile"},
	}

	sess := New(cfg, mockRun, store).WithTooling(toolingMock)
	err := sess.Start(context.Background())
	if err == nil {
		t.Fatal("Start debió fallar porque el preset intentó sobrescribir el servidor 'gz-ia'")
	}
	if !strings.Contains(err.Error(), "gz-ia") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestSessionStart_ToolkitPresetCollisions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}

	// 1. Colisión de Env
	toolingMockEnv := &mockToolingService{
		composeToolkitsFunc: func(ctx context.Context, ids []string) (*tooling.ComposedTooling, error) {
			return &tooling.ComposedTooling{
				Env: map[string]string{"API_KEY": "val1"},
			}, nil
		},
		getPresetFunc: func(ctx context.Context, name string) (*tooling.Preset, error) {
			return &tooling.Preset{
				Name: name,
				Env:  map[string]string{"API_KEY": "val2"},
			}, nil
		},
	}

	sessEnv := New(Config{ID: "coll_env", WorkingDir: tmpDir, Profiles: []string{"p1"}}, mockRun, store).WithTooling(toolingMockEnv)
	err := sessEnv.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "colisión de variables de entorno") {
		t.Errorf("se esperaba error de colisión de variables de entorno, obtenido: %v", err)
	}

	// 2. Colisión de MCPServers
	toolingMockMCP := &mockToolingService{
		composeToolkitsFunc: func(ctx context.Context, ids []string) (*tooling.ComposedTooling, error) {
			return &tooling.ComposedTooling{
				MCPServers: map[string]any{"my-srv": "cfg1"},
			}, nil
		},
		getPresetFunc: func(ctx context.Context, name string) (*tooling.Preset, error) {
			return &tooling.Preset{
				Name:       name,
				MCPServers: map[string]any{"my-srv": "cfg2"},
			}, nil
		},
	}

	sessMCP := New(Config{ID: "coll_mcp", WorkingDir: tmpDir, Profiles: []string{"p1"}}, mockRun, store).WithTooling(toolingMockMCP)
	err = sessMCP.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "colisión de servidores MCP") {
		t.Errorf("se esperaba error de colisión de servidores MCP, obtenido: %v", err)
	}
}

func TestSessionStart_NonIsolatedUnprojectsOnExit(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	sessID := "non_isolated_cleanup"

	// Crear manifest para la sesión
	manifest := workspace.NewManifest(sessID)
	manifest.CreatedFiles = []string{".agents/toolkits/TOOLKIT_A-AGENTS.md"}
	if err := workspace.SaveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("SaveManifest falló: %v", err)
	}

	toolingMock := &mockToolingService{}

	// Mock workspace no aislado
	mockWS := &mockWorkspaceProvider{
		isGitAvail: false,
		prepareWS: &workspace.Workspace{
			WorkingDir: tmpDir,
			TargetDir:  tmpDir,
			IsIsolated: false,
		},
	}

	cfg := Config{
		ID:         sessID,
		WorkingDir: tmpDir,
		Profiles:   []string{"base-profile"},
	}

	sess := New(cfg, mockRun, store).
		WithWorkspace(mockWS).
		WithTooling(toolingMock)

	err := sess.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() falló: %v", err)
	}

	if !toolingMock.unprojectCalled {
		t.Errorf("Unproject debió ser llamado automáticamente al finalizar la sesión no aislada")
	}
}

type mockLoggerService struct {
	emitted []*logger.Event
	events  []logger.Event
}

func (m *mockLoggerService) Emit(ctx context.Context, evt *logger.Event) error {
	m.emitted = append(m.emitted, evt)
	return nil
}

func (m *mockLoggerService) GetEvents(ctx context.Context, sessionID string) ([]logger.Event, error) {
	return m.events, nil
}

func (m *mockLoggerService) Watch(ctx context.Context, sessionID string) (<-chan logger.Event, error) {
	ch := make(chan logger.Event)
	close(ch)
	return ch, nil
}

func TestSession_StartEventMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	mockLog := &mockLoggerService{}

	cfg := Config{
		ID:              "meta-sess-1",
		Provider:        "agy",
		WorkingDir:      tmpDir,
		InitialPrompt:   "analizar el proyecto",
		Profiles:        []string{"gz-ia", "maintainer"},
		PermissionLevel: PermissionAutonomous,
	}

	sess := New(cfg, mockRun, store).WithLogger(mockLog)
	_ = sess.Start(context.Background())

	if len(mockLog.emitted) < 1 {
		t.Fatal("Se esperaba al menos un evento emitido por Start()")
	}

	startEvt := mockLog.emitted[0]
	if startEvt.Stage != logger.StagePending {
		t.Errorf("Stage esperado StagePending, obtenido %s", startEvt.Stage)
	}
	if startEvt.Metadata == nil {
		t.Fatal("Metadata del evento START no debe ser nil")
	}

	if v, ok := startEvt.Metadata["provider"].(string); !ok || v != "agy" {
		t.Errorf("Metadata 'provider' esperado 'agy', obtenido %v", startEvt.Metadata["provider"])
	}

	toolkits, ok := startEvt.Metadata["toolkits"]
	if !ok {
		t.Error("Metadata 'toolkits' no presente en evento START")
	} else if toolkits == nil {
		t.Error("Metadata 'toolkits' no debe ser nil")
	}

	if v, ok := startEvt.Metadata["branch"].(string); !ok || v != "harness/meta-sess-1" {
		t.Errorf("Metadata 'branch' esperado 'harness/meta-sess-1', obtenido %v", startEvt.Metadata["branch"])
	}

	if v, ok := startEvt.Metadata["initial_prompt"].(string); !ok || v != "analizar el proyecto" {
		t.Errorf("Metadata 'initial_prompt' esperado 'analizar el proyecto', obtenido %v", startEvt.Metadata["initial_prompt"])
	}

	if v, ok := startEvt.Metadata["working_dir"].(string); !ok || v != tmpDir {
		t.Errorf("Metadata 'working_dir' esperado '%s', obtenido %v", tmpDir, startEvt.Metadata["working_dir"])
	}
}

func TestSession_StartEventMetadata_PromptTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	mockRun := &mockRunner{}
	mockLog := &mockLoggerService{}

	longPrompt := strings.Repeat("a", 250)
	cfg := Config{
		ID:              "trunc-sess",
		Provider:        "agy",
		WorkingDir:      tmpDir,
		InitialPrompt:   longPrompt,
		PermissionLevel: PermissionAutonomous,
	}

	sess := New(cfg, mockRun, store).WithLogger(mockLog)
	_ = sess.Start(context.Background())

	if len(mockLog.emitted) < 1 {
		t.Fatal("Se esperaba al menos un evento")
	}

	startEvt := mockLog.emitted[0]
	prompt, ok := startEvt.Metadata["initial_prompt"].(string)
	if !ok {
		t.Fatal("initial_prompt no presente en metadata")
	}
	if len(prompt) > 203 {
		t.Errorf("prompt no truncado correctamente: longitud %d", len(prompt))
	}
	if !strings.HasSuffix(prompt, "...") {
		t.Errorf("prompt truncado debe terminar con '...', obtenido: %s", prompt[len(prompt)-5:])
	}
}

