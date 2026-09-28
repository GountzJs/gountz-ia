package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/metrics"
	"gz-ia/internal/features/tooling"
	"gz-ia/internal/features/vault"
	"gz-ia/internal/features/workspace"
)

type mockServiceRunner struct {
	lastBinary string
	lastArgs   []string
	lastDir    string
	lastEnv    []string
	exitCode   int
	err        error
	onStartPID int
}

func (m *mockServiceRunner) SetEnv(env []string) {
	m.lastEnv = env
}

func (m *mockServiceRunner) Run(ctx context.Context, binary string, args []string, dir string, onStart func(pid int)) (int, error) {
	m.lastBinary = binary
	m.lastArgs = args
	m.lastDir = dir
	if onStart != nil {
		pid := m.onStartPID
		if pid == 0 {
			pid = 54321
		}
		onStart(pid)
	}
	return m.exitCode, m.err
}

type mockServiceKiller struct {
	lastPID int
	err     error
}

func (k *mockServiceKiller) Kill(pid int) error {
	k.lastPID = pid
	return k.err
}

type mockServiceWorkspace struct {
	isGitAvailable bool
	resolvedRoot   string
	cleanedWT      string
	cleanedBr      string
	diffReturn     string
	diffErr        error
	mergeReturn    *workspace.MergeResult
	mergeErr       error
}

func (m *mockServiceWorkspace) IsGitAvailable(ctx context.Context, dir string) bool {
	return m.isGitAvailable
}

func (m *mockServiceWorkspace) ResolveProjectRoot(ctx context.Context, dir string) string {
	if m.resolvedRoot != "" {
		return m.resolvedRoot
	}
	return dir
}

func (m *mockServiceWorkspace) Prepare(ctx context.Context, sessionID string, baseDir string) (*workspace.Workspace, error) {
	return &workspace.Workspace{
		WorkingDir: baseDir,
		TargetDir:  baseDir,
		IsIsolated: m.isGitAvailable,
	}, nil
}

func (m *mockServiceWorkspace) Cleanup(ctx context.Context, ws *workspace.Workspace) error {
	return nil
}

func (m *mockServiceWorkspace) CleanupWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string) error {
	m.cleanedWT = worktreeDir
	m.cleanedBr = branchName
	return nil
}

func (m *mockServiceWorkspace) DiffWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string, statOnly bool) (string, error) {
	return m.diffReturn, m.diffErr
}

func (m *mockServiceWorkspace) MergeWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string, squash bool, noCommit bool) (*workspace.MergeResult, error) {
	return m.mergeReturn, m.mergeErr
}

func (m *mockServiceWorkspace) Prune(ctx context.Context, baseDir string, activeSessionIDs []string) (*workspace.PruneReport, error) {
	return &workspace.PruneReport{}, nil
}

func TestService_StartChat_Default(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}
	killer := &mockServiceKiller{}
	ws := &mockServiceWorkspace{isGitAvailable: false}

	svc := NewService(tmpDir,
		WithStore(store),
		WithRunner(runner),
		WithKiller(killer),
		WithWorkspace(ws),
	)

	var launchedID string
	var launchedIso bool
	req := StartChatRequest{
		InitialPrompt: "test prompt",
		OnLaunch: func(id string, isIsolated bool) {
			launchedID = id
			launchedIso = isIsolated
		},
	}

	err := svc.StartChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StartChat retornó error inesperado: %v", err)
	}

	if launchedID == "" {
		t.Error("OnLaunch no recibió ID de sesión")
	}
	if launchedIso {
		t.Error("isIsolated debería ser false")
	}

	if runner.lastBinary != "agy" {
		t.Errorf("runner esperó binario 'agy', obtuvo '%s'", runner.lastBinary)
	}
	if runner.lastDir != tmpDir {
		t.Errorf("runner esperó dir '%s', obtuvo '%s'", tmpDir, runner.lastDir)
	}

	// Verificar registro guardado en el almacén
	rec, err := svc.GetRecord(context.Background(), launchedID)
	if err != nil {
		t.Fatalf("no se pudo recuperar el registro guardado: %v", err)
	}
	if rec.Status != StatusCompleted {
		t.Errorf("estado esperado '%s', obtenido '%s'", StatusCompleted, rec.Status)
	}
	if rec.PID != 54321 {
		t.Errorf("PID esperado 54321, obtenido %d", rec.PID)
	}
}

func TestService_StartChat_CustomOptions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}
	ws := &mockServiceWorkspace{isGitAvailable: true}

	svc := NewService(tmpDir,
		WithStore(store),
		WithRunner(runner),
		WithWorkspace(ws),
	)

	req := StartChatRequest{
		ID:              "custom_id_123",
		WorkingDir:      tmpDir,
		InitialPrompt:   "hacer deploy",
		PermissionLevel: PermissionReadOnly,
		BinaryPath:      "agy_custom",
	}

	err := svc.StartChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StartChat retornó error: %v", err)
	}

	if runner.lastBinary != "agy_custom" {
		t.Errorf("binario esperado 'agy_custom', obtenido '%s'", runner.lastBinary)
	}

	expectedArgs := []string{"--mode", "plan", "-i", "hacer deploy"}
	if len(runner.lastArgs) != len(expectedArgs) {
		t.Fatalf("cantidad de argumentos inesperada: %v", runner.lastArgs)
	}
	for i, arg := range expectedArgs {
		if runner.lastArgs[i] != arg {
			t.Errorf("arg[%d] esperado '%s', obtenido '%s'", i, arg, runner.lastArgs[i])
		}
	}
}

func TestService_StartChat_RunnerError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{err: errors.New("falla agy")}

	svc := NewService(tmpDir,
		WithStore(store),
		WithRunner(runner),
	)

	err := svc.StartChat(context.Background(), StartChatRequest{
		ID: "fail_sess",
	})
	if err == nil {
		t.Fatal("se esperaba error y se obtuvo nil")
	}

	rec, getErr := svc.GetRecord(context.Background(), "fail_sess")
	if getErr != nil {
		t.Fatalf("error al obtener sesión: %v", getErr)
	}
	if rec.Status != StatusFailed {
		t.Errorf("estado esperado '%s', obtenido '%s'", StatusFailed, rec.Status)
	}
}

func TestService_List(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	_ = store.Save(&SessionRecord{
		ID:        "s1",
		Status:    StatusCompleted,
		StartedAt: time.Now().Add(-1 * time.Hour),
	})
	_ = store.Save(&SessionRecord{
		ID:        "s2",
		Status:    StatusRunning,
		StartedAt: time.Now(),
	})

	svc := NewService(tmpDir, WithStore(store))

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List falló: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 sesiones, obtenidas: %d", len(list))
	}
	// s2 es más reciente
	if list[0].ID != "s2" {
		t.Errorf("se esperaba 's2' primero, obtenido '%s'", list[0].ID)
	}
}

func TestService_GetRecord(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	_ = store.Save(&SessionRecord{
		ID:     "existing_01",
		Status: StatusCompleted,
	})

	svc := NewService(tmpDir, WithStore(store))

	rec, err := svc.GetRecord(context.Background(), "existing_01")
	if err != nil {
		t.Fatalf("GetRecord falló: %v", err)
	}
	if rec.ID != "existing_01" {
		t.Errorf("ID obtenido '%s', esperado 'existing_01'", rec.ID)
	}

	recSess, err := svc.GetSession(context.Background(), "existing_01")
	if err != nil || recSess.ID != "existing_01" {
		t.Errorf("GetSession falló: %v", err)
	}

	_, err = svc.GetRecord(context.Background(), "no_exist")
	if err == nil {
		t.Error("se esperaba error para sesión inexistente")
	}
}

func TestService_Kill(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	killer := &mockServiceKiller{}

	_ = store.Save(&SessionRecord{
		ID:        "running_01",
		PID:       4321,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	})

	svc := NewService(tmpDir, WithStore(store), WithKiller(killer))

	err := svc.Kill(context.Background(), "running_01")
	if err != nil {
		t.Fatalf("Kill falló: %v", err)
	}

	if killer.lastPID != 4321 {
		t.Errorf("killer esperaba PID 4321, obtuvo %d", killer.lastPID)
	}

	rec, _ := svc.GetRecord(context.Background(), "running_01")
	if rec.Status != StatusKilled {
		t.Errorf("estado esperado '%s', obtenido '%s'", StatusKilled, rec.Status)
	}

	// Matar sesión que no está corriendo debe fallar
	err = svc.Kill(context.Background(), "running_01")
	if err == nil {
		t.Error("matar sesión no activa debe retornar error")
	}
}

func TestService_Kill_KillerError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	killer := &mockServiceKiller{err: errors.New("permiso denegado")}

	_ = store.Save(&SessionRecord{
		ID:        "running_err",
		PID:       8888,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	})

	svc := NewService(tmpDir, WithStore(store), WithKiller(killer))

	err := svc.Kill(context.Background(), "running_err")
	if err == nil {
		t.Fatal("Kill debió retornar error si killer falla")
	}
}

func TestService_Resume(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}

	_ = store.Save(&SessionRecord{
		ID:              "resume_01",
		Status:          StatusCompleted,
		PermissionLevel: PermissionAutonomous,
		WorkingDir:      tmpDir,
		StartedAt:       time.Now(),
	})

	svc := NewService(tmpDir, WithStore(store), WithRunner(runner))

	err := svc.Resume(context.Background(), "resume_01")
	if err != nil {
		t.Fatalf("Resume falló: %v", err)
	}

	hasContinue := false
	for _, arg := range runner.lastArgs {
		if arg == "--continue" {
			hasContinue = true
			break
		}
	}
	if !hasContinue {
		t.Errorf("se esperaba bandera '--continue', argumentos: %v", runner.lastArgs)
	}

	// Reanudar sesión activa debe fallar
	_ = store.Save(&SessionRecord{
		ID:        "already_run",
		Status:    StatusRunning,
		StartedAt: time.Now(),
	})
	err = svc.Resume(context.Background(), "already_run")
	if err == nil {
		t.Error("reanudar sesión ya activa debe fallar")
	}
}

func TestService_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	ws := &mockServiceWorkspace{}
	wtDir := filepath.Join(tmpDir, ".harness", "worktrees", "del_wt")

	_ = store.Save(&SessionRecord{
		ID:          "del_01",
		Status:      StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: wtDir,
		BranchName:  "harness/del_01",
		StartedAt:   time.Now(),
	})

	svc := NewService(tmpDir, WithStore(store), WithWorkspace(ws))

	err := svc.Delete(context.Background(), "del_01")
	if err != nil {
		t.Fatalf("Delete falló: %v", err)
	}

	if ws.cleanedWT != wtDir {
		t.Errorf("se esperaba limpieza de worktree en '%s', obtenida '%s'", wtDir, ws.cleanedWT)
	}

	_, err = svc.GetRecord(context.Background(), "del_01")
	if err == nil {
		t.Error("la sesión debe haber sido eliminada del almacén")
	}
}

func TestService_Delete_RunningSession(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	killer := &mockServiceKiller{}

	_ = store.Save(&SessionRecord{
		ID:        "del_running",
		PID:       7777,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	})

	svc := NewService(tmpDir, WithStore(store), WithKiller(killer))

	err := svc.Delete(context.Background(), "del_running")
	if err != nil {
		t.Fatalf("Delete falló: %v", err)
	}

	if killer.lastPID != 7777 {
		t.Errorf("Delete debió terminar PID 7777, obtenido %d", killer.lastPID)
	}
}

func TestService_Path(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	wtDir := filepath.Join(tmpDir, ".harness", "worktrees", "path_sess")

	_ = store.Save(&SessionRecord{
		ID:          "p_iso",
		Status:      StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: wtDir,
	})
	_ = store.Save(&SessionRecord{
		ID:         "p_dir",
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	svc := NewService(tmpDir, WithStore(store))

	pIso, err := svc.Path(context.Background(), "p_iso")
	if err != nil {
		t.Fatalf("Path p_iso falló: %v", err)
	}
	if pIso != filepath.Clean(wtDir) {
		t.Errorf("Path aislado esperado '%s', obtenido '%s'", filepath.Clean(wtDir), pIso)
	}

	pDir, err := svc.Path(context.Background(), "p_dir")
	if err != nil {
		t.Fatalf("Path p_dir falló: %v", err)
	}
	if pDir != filepath.Clean(tmpDir) {
		t.Errorf("Path directo esperado '%s', obtenido '%s'", filepath.Clean(tmpDir), pDir)
	}
}

func TestService_Diff(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	ws := &mockServiceWorkspace{diffReturn: "+added line"}

	_ = store.Save(&SessionRecord{
		ID:          "diff_iso",
		Status:      StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: filepath.Join(tmpDir, "wt"),
		BranchName:  "harness/diff_iso",
	})
	_ = store.Save(&SessionRecord{
		ID:         "diff_dir",
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	svc := NewService(tmpDir, WithStore(store), WithWorkspace(ws))

	out, err := svc.Diff(context.Background(), "diff_iso", false)
	if err != nil {
		t.Fatalf("Diff falló: %v", err)
	}
	if out != "+added line" {
		t.Errorf("salida esperada '+added line', obtenida '%s'", out)
	}

	outDir, err := svc.Diff(context.Background(), "diff_dir", false)
	if err != nil {
		t.Fatalf("Diff directo falló: %v", err)
	}
	if !strings.Contains(outDir, "modo directo") {
		t.Errorf("se esperaba mensaje sobre modo directo, obtenido '%s'", outDir)
	}
}

func TestService_Merge(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	expectedRes := &workspace.MergeResult{
		Message:         "merge ok",
		FilesIntegrated: []string{"file1.go"},
	}
	ws := &mockServiceWorkspace{mergeReturn: expectedRes}

	_ = store.Save(&SessionRecord{
		ID:          "merge_iso",
		Status:      StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: filepath.Join(tmpDir, "wt"),
		BranchName:  "harness/merge_iso",
	})
	_ = store.Save(&SessionRecord{
		ID:         "merge_dir",
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	svc := NewService(tmpDir, WithStore(store), WithWorkspace(ws))

	res, err := svc.Merge(context.Background(), "merge_iso", MergeOptions{Squash: true, NoCommit: true})
	if err != nil {
		t.Fatalf("Merge falló: %v", err)
	}
	if res != expectedRes {
		t.Errorf("MergeResult obtenido %v, esperado %v", res, expectedRes)
	}

	_, err = svc.Merge(context.Background(), "merge_dir", MergeOptions{})
	if err == nil {
		t.Error("Merge en sesión directa debió fallar")
	}
}

func TestService_Read_And_Get_Parity(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	expectedRes := &workspace.MergeResult{
		Message:         "Integrated",
		FilesIntegrated: []string{"file.go"},
	}
	ws := &mockServiceWorkspace{
		diffReturn:  "diff --git a/test.go b/test.go\n+new line",
		mergeReturn: expectedRes,
	}

	_ = store.Save(&SessionRecord{
		ID:          "wt_iso",
		Status:      StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: filepath.Join(tmpDir, "wt"),
		BranchName:  "harness/wt_iso",
	})
	_ = store.Save(&SessionRecord{
		ID:         "wt_dir",
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	svc := NewService(tmpDir, WithStore(store), WithWorkspace(ws))

	// Read tests
	diffOut, err := svc.Read(context.Background(), "wt_iso", false)
	if err != nil {
		t.Fatalf("Read falló: %v", err)
	}
	if !strings.Contains(diffOut, "+new line") {
		t.Errorf("Read diff inesperado: %s", diffOut)
	}

	directOut, err := svc.Read(context.Background(), "wt_dir", false)
	if err != nil {
		t.Fatalf("Read directo falló: %v", err)
	}
	if !strings.Contains(directOut, "modo directo") {
		t.Errorf("Read directo inesperado: %s", directOut)
	}

	// Get tests
	res, err := svc.Get(context.Background(), "wt_iso", MergeOptions{Squash: true, NoCommit: true})
	if err != nil {
		t.Fatalf("Get falló: %v", err)
	}
	if res != expectedRes {
		t.Errorf("Get result obtenido %v, esperado %v", res, expectedRes)
	}

	_, err = svc.Get(context.Background(), "wt_dir", MergeOptions{})
	if err == nil {
		t.Error("Get en sesión directa debió fallar")
	}
}

func TestService_NewService_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)
	if svc == nil {
		t.Fatal("NewService() retornó nil")
	}

	// Debe poder listar sesiones del directorio vacío sin errores
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List falló en directorio vacío: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("se esperaba lista vacía, obtenida: %d", len(list))
	}
}

func TestService_NilStoreErrors(t *testing.T) {
	svc := &sessionService{} // store is nil
	ctx := context.Background()

	if _, err := svc.List(ctx); err == nil {
		t.Error("List con store nil debió fallar")
	}
	if _, err := svc.GetRecord(ctx, "id"); err == nil {
		t.Error("GetRecord con store nil debió fallar")
	}
	if _, err := svc.GetSession(ctx, "id"); err == nil {
		t.Error("GetSession con store nil debió fallar")
	}
	if _, err := svc.Get(ctx, "id", MergeOptions{}); err == nil {
		t.Error("Get con store nil debió fallar")
	}
	if _, err := svc.Read(ctx, "id", false); err == nil {
		t.Error("Read con store nil debió fallar")
	}
	if err := svc.Kill(ctx, "id"); err == nil {
		t.Error("Kill con store nil debió fallar")
	}
	if err := svc.Resume(ctx, "id"); err == nil {
		t.Error("Resume con store nil debió fallar")
	}
	if err := svc.Delete(ctx, "id"); err == nil {
		t.Error("Delete con store nil debió fallar")
	}
	if _, err := svc.Path(ctx, "id"); err == nil {
		t.Error("Path con store nil debió fallar")
	}
	if _, err := svc.Diff(ctx, "id", false); err == nil {
		t.Error("Diff con store nil debió fallar")
	}
	if _, err := svc.Merge(ctx, "id", MergeOptions{}); err == nil {
		t.Error("Merge con store nil debió fallar")
	}
	if _, err := svc.Metrics(ctx, "id"); err == nil {
		t.Error("Metrics con store nil debió fallar")
	}
}

type mockMetricsService struct {
	lastSessionID string
	lastWorkDir   string
	returnMetrics *metrics.SessionMetrics
	returnErr     error
}

func (m *mockMetricsService) GetMetrics(ctx context.Context, sessionID string, workDir string) (*metrics.SessionMetrics, error) {
	m.lastSessionID = sessionID
	m.lastWorkDir = workDir
	return m.returnMetrics, m.returnErr
}

func TestService_Metrics(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	rec := &SessionRecord{
		ID:          "sess-metrics-1",
		Status:      StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: filepath.Join(tmpDir, "worktree"),
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}

	expectedMetrics := &metrics.SessionMetrics{
		SessionID:      "sess-metrics-1",
		ConversationID: "conv-123",
		StepsCount:     5,
		Tokens: metrics.TokenUsage{
			PromptTokens:     10,
			ThinkingTokens:   20,
			CompletionTokens: 30,
			TotalTokens:      60,
		},
		ToolCalls: map[string]int{"run_command": 2},
	}

	mockMetrics := &mockMetricsService{
		returnMetrics: expectedMetrics,
	}

	svc := NewService(tmpDir, WithStore(store), WithMetrics(mockMetrics))

	ctx := context.Background()
	res, err := svc.Metrics(ctx, "sess-metrics-1")
	if err != nil {
		t.Fatalf("Metrics falló: %v", err)
	}

	if mockMetrics.lastSessionID != "sess-metrics-1" {
		t.Errorf("lastSessionID esperado 'sess-metrics-1', obtenido '%s'", mockMetrics.lastSessionID)
	}
	if mockMetrics.lastWorkDir != filepath.Join(tmpDir, "worktree") {
		t.Errorf("lastWorkDir esperado '%s', obtenido '%s'", filepath.Join(tmpDir, "worktree"), mockMetrics.lastWorkDir)
	}
	if res.StepsCount != 5 || res.Tokens.TotalTokens != 60 {
		t.Errorf("métricas inesperadas: %+v", res)
	}

	// Caso sesión inexistente
	if _, err := svc.Metrics(ctx, "no-existe"); err == nil {
		t.Error("se esperaba error para sesión inexistente")
	}
}

func TestService_Logger_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(filepath.Join(tmpDir, ".harness", "sessions"))
	runner := &mockServiceRunner{}
	loggerSvc := logger.NewService(tmpDir)

	svc := NewService(tmpDir, WithStore(store), WithRunner(runner), WithLogger(loggerSvc))
	ctx := context.Background()

	// 1. LogEvent manual y GetEvents
	sessID := "sess-log-test"
	manualEvt := &logger.Event{
		SessionID: sessID,
		AgentID:   "sub-1",
		Role:      "tester",
		Action:    "Ejecutando pruebas unitarias",
		Stage:     logger.StagePending,
	}

	if err := svc.LogEvent(ctx, manualEvt); err != nil {
		t.Fatalf("LogEvent falló: %v", err)
	}

	events, err := svc.GetEvents(ctx, sessID)
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Se esperaba 1 evento, se obtuvieron %d", len(events))
	}
	if events[0].Action != "Ejecutando pruebas unitarias" {
		t.Errorf("Acción de evento inesperada: %s", events[0].Action)
	}

	// 2. StartChat emite eventos automáticos de inicio y fin
	chatSessID := "sess-chat-lifecycle"
	req := StartChatRequest{
		ID:         chatSessID,
		WorkingDir: tmpDir,
	}
	if err := svc.StartChat(ctx, req); err != nil {
		t.Fatalf("StartChat falló: %v", err)
	}

	chatEvents, err := svc.GetEvents(ctx, chatSessID)
	if err != nil {
		t.Fatalf("GetEvents para chat falló: %v", err)
	}
	if len(chatEvents) != 2 {
		t.Fatalf("Se esperaban 2 eventos de ciclo de vida (inicio y fin), se obtuvieron %d", len(chatEvents))
	}
	if chatEvents[0].Stage != logger.StagePending || chatEvents[0].Action != "Sesión de chat iniciada" {
		t.Errorf("Evento de inicio inesperado: %+v", chatEvents[0])
	}
	if chatEvents[1].Stage != logger.StageFinish || chatEvents[1].Status == nil || *chatEvents[1].Status != logger.StatusOK {
		t.Errorf("Evento de finalización inesperado: %+v", chatEvents[1])
	}

	// 3. Resume emite evento de reanudación
	// Marcamos la sesión como completed para poder reanudarla
	rec, _ := store.Get(chatSessID)
	rec.Status = StatusCompleted
	_ = store.Save(rec)

	if err := svc.Resume(ctx, chatSessID); err != nil {
		t.Fatalf("Resume falló: %v", err)
	}

	resumedEvents, err := svc.GetEvents(ctx, chatSessID)
	if err != nil {
		t.Fatalf("GetEvents post-resume falló: %v", err)
	}
	if len(resumedEvents) != 4 {
		t.Fatalf("Se esperaban 4 eventos en total tras reanudar, se obtuvieron %d", len(resumedEvents))
	}
	if resumedEvents[2].Action != "Sesión de chat reanudada" {
		t.Errorf("Evento de reanudación inesperado: %+v", resumedEvents[2])
	}

	// 4. Kill emite evento de interrupción / kill
	killSessID := "sess-kill-lifecycle"
	_ = store.Save(&SessionRecord{
		ID:        killSessID,
		PID:       99999,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	})
	mockKiller := &mockServiceKiller{}
	svcWithKiller := NewService(tmpDir, WithStore(store), WithKiller(mockKiller), WithLogger(loggerSvc))

	if err := svcWithKiller.Kill(ctx, killSessID); err != nil {
		t.Fatalf("Kill falló: %v", err)
	}

	killEvents, err := svc.GetEvents(ctx, killSessID)
	if err != nil {
		t.Fatalf("GetEvents post-kill falló: %v", err)
	}
	if len(killEvents) != 1 {
		t.Fatalf("Se esperaba 1 evento de kill, se obtuvieron %d", len(killEvents))
	}
	if killEvents[0].Stage != logger.StageFinish || killEvents[0].Status == nil || *killEvents[0].Status != logger.StatusFailed {
		t.Errorf("Evento de kill inesperado: %+v", killEvents[0])
	}
}

func TestService_StartChat_WithProvider(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}
	svc := NewService(tmpDir, WithStore(store), WithRunner(runner))

	req := StartChatRequest{
		ID:              "claude_sess",
		Provider:        "claude",
		WorkingDir:      tmpDir,
		InitialPrompt:   "hola claude",
		PermissionLevel: PermissionAutonomous,
	}

	err := svc.StartChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StartChat con claude falló: %v", err)
	}

	if runner.lastBinary != "claude" {
		t.Errorf("binario esperado 'claude', obtenido '%s'", runner.lastBinary)
	}

	rec, err := svc.GetRecord(context.Background(), "claude_sess")
	if err != nil {
		t.Fatalf("GetRecord falló: %v", err)
	}
	if rec.Provider != "claude" {
		t.Errorf("Provider en SessionRecord esperado 'claude', obtenido '%s'", rec.Provider)
	}
}

func TestService_Resume_MultiDriver(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}
	svc := NewService(tmpDir, WithStore(store), WithRunner(runner))

	// Reanudar claude
	_ = store.Save(&SessionRecord{
		ID:              "resume_claude",
		Provider:        "claude",
		Status:          StatusCompleted,
		PermissionLevel: PermissionAutonomous,
		WorkingDir:      tmpDir,
		StartedAt:       time.Now(),
	})

	err := svc.Resume(context.Background(), "resume_claude")
	if err != nil {
		t.Fatalf("Resume claude falló: %v", err)
	}
	if runner.lastBinary != "claude" {
		t.Errorf("binario esperado 'claude', obtenido '%s'", runner.lastBinary)
	}
	hasResumeFlag := false
	for _, arg := range runner.lastArgs {
		if arg == "--resume" {
			hasResumeFlag = true
		}
	}
	if !hasResumeFlag {
		t.Errorf("se esperaba '--resume' para claude, obtenido %v", runner.lastArgs)
	}

	// Reanudar opencode
	_ = store.Save(&SessionRecord{
		ID:              "resume_opencode",
		Provider:        "opencode",
		Status:          StatusCompleted,
		PermissionLevel: PermissionAutonomous,
		WorkingDir:      tmpDir,
		StartedAt:       time.Now(),
	})

	err = svc.Resume(context.Background(), "resume_opencode")
	if err != nil {
		t.Fatalf("Resume opencode falló: %v", err)
	}
	if runner.lastBinary != "opencode" {
		t.Errorf("binario esperado 'opencode', obtenido '%s'", runner.lastBinary)
	}
	hasContinueFlag := false
	for _, arg := range runner.lastArgs {
		if arg == "--continue" {
			hasContinueFlag = true
		}
	}
	if !hasContinueFlag {
		t.Errorf("se esperaba '--continue' para opencode, obtenido %v", runner.lastArgs)
	}
}

func TestService_StartChat_WithProfiles(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}

	toolingSvc := tooling.NewService(filepath.Join(tmpDir, "tooling"), tmpDir)
	_, err := toolingSvc.CreateToolkit(context.Background(), tooling.CreateToolkitRequest{
		ID:          "frontend",
		Description: "Frontend toolkit",
	})
	if err != nil {
		t.Fatalf("CreateToolkit falló: %v", err)
	}

	svc := NewService(tmpDir, WithStore(store), WithRunner(runner), WithTooling(toolingSvc))

	req := StartChatRequest{
		ID:         "prof_sess_1",
		Provider:   "agy",
		WorkingDir: tmpDir,
		Profiles:   []string{"frontend"},
	}

	err = svc.StartChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StartChat con toolkits falló: %v", err)
	}

	rec, err := store.Get("prof_sess_1")
	if err != nil {
		t.Fatalf("Get fallo: %v", err)
	}
	if len(rec.Profiles) != 1 || rec.Profiles[0] != "frontend" {
		t.Errorf("profiles esperados ['frontend'], obtenidos %v", rec.Profiles)
	}
}

func TestService_Vault_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}
	vSvc := vault.NewService(tmpDir)
	ctx := context.Background()

	_ = vSvc.Set(ctx, "MY_CUSTOM_SECRET", "secret-value-xyz")

	svc := NewService(tmpDir, WithStore(store), WithRunner(runner), WithVault(vSvc))

	err := svc.StartChat(ctx, StartChatRequest{
		ID:         "vault_sess_1",
		Provider:   "agy",
		WorkingDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("StartChat con vault falló: %v", err)
	}

	foundSecret := false
	for _, envStr := range runner.lastEnv {
		if strings.HasPrefix(envStr, "MY_CUSTOM_SECRET=") {
			foundSecret = true
			if envStr != "MY_CUSTOM_SECRET=secret-value-xyz" {
				t.Errorf("Valor inesperado inyectado: %s", envStr)
			}
		}
	}
	if !foundSecret {
		t.Errorf("La variable MY_CUSTOM_SECRET no fue inyectada en el runner")
	}
}

func TestService_StartChat_DefaultOrchestratorPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	runner := &mockServiceRunner{}
	ctx := context.Background()

	svc := NewService(tmpDir, WithStore(store), WithRunner(runner))

	// Iniciar chat sin InitialPrompt
	req := StartChatRequest{
		ID:         "prompt_sess_1",
		Provider:   "agy",
		WorkingDir: tmpDir,
	}

	err := svc.StartChat(ctx, req)
	if err != nil {
		t.Fatalf("StartChat falló: %v", err)
	}

	rec, err := store.Get("prompt_sess_1")
	if err != nil {
		t.Fatalf("Get falló: %v", err)
	}

	if rec.InitialPrompt != DefaultOrchestratorPrompt {
		t.Errorf("InitialPrompt esperado '%s', obtenido '%s'", DefaultOrchestratorPrompt, rec.InitialPrompt)
	}

	// Comprobar que en los args pasados al runner está el DefaultOrchestratorPrompt
	hasPromptArg := false
	for i, arg := range runner.lastArgs {
		if arg == "-i" && i+1 < len(runner.lastArgs) && runner.lastArgs[i+1] == DefaultOrchestratorPrompt {
			hasPromptArg = true
			break
		}
	}
	if !hasPromptArg {
		t.Errorf("No se pasó DefaultOrchestratorPrompt al runner: %v", runner.lastArgs)
	}
}

func TestService_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	ctx := context.Background()

	sessID := "cleanup_test_sess"
	createdRel := "created_by_gz_ia.txt"
	createdPath := filepath.Join(tmpDir, createdRel)
	_ = os.WriteFile(createdPath, []byte("temporary projected content"), 0644)

	manifest := workspace.NewManifest(sessID)
	manifest.CreatedFiles = []string{createdRel}
	if err := workspace.SaveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("SaveManifest falló: %v", err)
	}

	rec := &SessionRecord{
		ID:         sessID,
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	}
	_ = store.Save(rec)

	svc := NewService(tmpDir, WithStore(store))
	err := svc.Cleanup(ctx, sessID)
	if err != nil {
		t.Fatalf("Cleanup falló: %v", err)
	}

	if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
		t.Errorf("El archivo %s debió ser eliminado por Cleanup", createdPath)
	}
}

func TestService_Prune_UnprojectsTerminatedSessions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	ctx := context.Background()

	sessID := "prune_cleanup_sess"
	createdRel := "orphaned_proj_file.txt"
	createdPath := filepath.Join(tmpDir, createdRel)
	_ = os.WriteFile(createdPath, []byte("residual content"), 0644)

	manifest := workspace.NewManifest(sessID)
	manifest.CreatedFiles = []string{createdRel}
	if err := workspace.SaveManifest(tmpDir, manifest); err != nil {
		t.Fatalf("SaveManifest falló: %v", err)
	}

	rec := &SessionRecord{
		ID:         sessID,
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	}
	_ = store.Save(rec)

	svc := NewService(tmpDir, WithStore(store))
	res, err := svc.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune falló: %v", err)
	}
	if res == nil {
		t.Fatal("Prune retornó nil")
	}

	if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
		t.Errorf("El archivo %s debió ser desproyectado durante Prune", createdPath)
	}
}

func TestSessionService_Context(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	loggerSvc := logger.NewService(tmpDir)
	ctx := context.Background()

	sessID := "ctx-svc-test"
	_ = store.Save(&SessionRecord{
		ID:         sessID,
		Provider:   "agy",
		Status:     StatusCompleted,
		WorkingDir: tmpDir,
		Profiles:   []string{"gz-ia"},
		StartedAt:  time.Now().Add(-1 * time.Hour),
	})

	st := logger.StatusOK
	_ = loggerSvc.Emit(ctx, &logger.Event{
		SessionID: sessID,
		AgentID:   "orchestrator",
		Role:      "orchestrator",
		Action:    "Sesión de chat iniciada",
		Stage:     logger.StagePending,
		Metadata: map[string]any{
			"provider":       "agy",
			"toolkits":       []string{"gz-ia"},
			"branch":         "harness/" + sessID,
			"initial_prompt": "analizar workspace",
			"working_dir":    tmpDir,
		},
	})
	_ = loggerSvc.Emit(ctx, &logger.Event{
		SessionID: sessID,
		AgentID:   "orchestrator",
		Role:      "orchestrator",
		Action:    "Sesión finalizada exitosamente",
		Stage:     logger.StageFinish,
		Status:    &st,
		Metadata: map[string]any{
			"exit_code":     0,
			"duration_s":    int64(3600),
			"files_changed": "internal/foo.go | 12 ++++",
		},
	})

	svc := NewService(tmpDir, WithStore(store), WithLogger(loggerSvc))

	sc, err := svc.Context(ctx, sessID)
	if err != nil {
		t.Fatalf("Context() falló: %v", err)
	}

	if sc.SessionID != sessID {
		t.Errorf("SessionID esperado '%s', obtenido '%s'", sessID, sc.SessionID)
	}
	if sc.Provider != "agy" {
		t.Errorf("Provider esperado 'agy', obtenido '%s'", sc.Provider)
	}
	if sc.Branch != "harness/"+sessID {
		t.Errorf("Branch esperado 'harness/%s', obtenido '%s'", sessID, sc.Branch)
	}
	if sc.InitialPrompt != "analizar workspace" {
		t.Errorf("InitialPrompt esperado 'analizar workspace', obtenido '%s'", sc.InitialPrompt)
	}
	if len(sc.Toolkits) == 0 {
		t.Error("Toolkits no debe ser vacío")
	}
	if sc.FilesChanged == "" {
		t.Error("FilesChanged debe estar poblado desde metadata del evento FINISH")
	}
	if len(sc.Events) != 2 {
		t.Errorf("Se esperaban 2 eventos, obtenidos %d", len(sc.Events))
	}
	if sc.Status != string(StatusCompleted) {
		t.Errorf("Status esperado '%s', obtenido '%s'", StatusCompleted, sc.Status)
	}

	_, err = svc.Context(ctx, "inexistente")
	if err == nil {
		t.Error("Context() debió retornar error para sesión inexistente")
	}
}

func TestSessionService_Context_NilDependencies(t *testing.T) {
	ctx := context.Background()

	svcNoStore := &sessionService{}
	if _, err := svcNoStore.Context(ctx, "id"); err == nil {
		t.Error("Context con store nil debió fallar")
	}

	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	svcNoLogger := &sessionService{store: store}
	if _, err := svcNoLogger.Context(ctx, "id"); err == nil {
		t.Error("Context con logger nil debió fallar")
	}
}

