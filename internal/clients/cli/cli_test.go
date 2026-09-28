package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/metrics"
	"gz-ia/internal/features/session"
	"gz-ia/internal/features/updater"
	"gz-ia/internal/features/workspace"
	"gz-ia/internal/version"
)

type mockSessionRunner struct {
	lastBinary string
	lastArgs   []string
	lastDir    string
	returnErr  error
}

func (m *mockSessionRunner) Run(ctx context.Context, binary string, args []string, dir string, onStart func(pid int)) (int, error) {
	m.lastBinary = binary
	m.lastArgs = args
	m.lastDir = dir
	if onStart != nil {
		onStart(8888)
	}
	exitCode := 0
	if m.returnErr != nil {
		exitCode = 1
	}
	return exitCode, m.returnErr
}

type mockKiller struct {
	lastPID int
	err     error
}

func (k *mockKiller) Kill(pid int) error {
	k.lastPID = pid
	return k.err
}

func TestRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("cmd --help falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gz-ia") {
		t.Errorf("la salida de ayuda no contiene 'gz-ia': %s", out)
	}
	if !strings.Contains(out, "start") {
		t.Errorf("la salida de ayuda no lista el comando 'start': %s", out)
	}
	if !strings.Contains(out, "chat") {
		t.Errorf("la salida de ayuda no lista el comando 'chat': %s", out)
	}
	if !strings.Contains(out, "session") {
		t.Errorf("la salida de ayuda no lista el comando 'session': %s", out)
	}
	if !strings.Contains(out, "toolkit") {
		t.Errorf("la salida de ayuda no lista el comando 'toolkit': %s", out)
	}
	if !strings.Contains(out, "version") {
		t.Errorf("la salida de ayuda no lista el comando 'version': %s", out)
	}
	if !strings.Contains(out, "update") {
		t.Errorf("la salida de ayuda no lista el comando 'update': %s", out)
	}
}

func TestVersionCmd(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("versionCmd falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gz-ia") {
		t.Errorf("salida de versión inesperada: %s", out)
	}
	if !strings.Contains(out, "v"+version.Version) {
		t.Errorf("salida de versión no contiene 'v%s': %s", version.Version, out)
	}
}

func TestStartCmd_Execution(t *testing.T) {
	originalRunTUI := runTUI
	defer func() { runTUI = originalRunTUI }()

	called := false
	runTUI = func() error {
		called = true
		return nil
	}

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("startCmd falló: %v", err)
	}

	if !called {
		t.Errorf("startCmd no ejecutó la función runTUI")
	}
}

func TestStartCmd_ErrorPropagation(t *testing.T) {
	originalRunTUI := runTUI
	defer func() { runTUI = originalRunTUI }()

	expectedErr := errors.New("error simulado en tui")
	runTUI = func() error {
		return expectedErr
	}

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"start"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("se esperaba error y se obtuvo nil")
	}
	if !strings.Contains(err.Error(), "error simulado en tui") {
		t.Errorf("error inesperado: obtenido %v, esperado %v", err, expectedErr)
	}
}

func TestRootCmd_DefaultInvokesStart(t *testing.T) {
	originalRunTUI := runTUI
	defer func() { runTUI = originalRunTUI }()

	called := false
	runTUI = func() error {
		called = true
		return nil
	}

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("rootCmd por defecto falló: %v", err)
	}

	if !called {
		t.Errorf("rootCmd sin argumentos no invocó start. Salida: %s", buf.String())
	}
}

func TestChatCmd_Execution(t *testing.T) {
	tmpDir := t.TempDir()
	originalRunner := defaultSessionRunner
	defer func() { defaultSessionRunner = originalRunner }()

	mock := &mockSessionRunner{}
	defaultSessionRunner = mock

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "--perm", "readonly", "--prompt", "analizar auth", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("chatCmd falló: %v", err)
	}

	if mock.lastBinary != "agy" {
		t.Errorf("binario esperado 'agy', obtenido '%s'", mock.lastBinary)
	}
	if mock.lastDir != tmpDir {
		t.Errorf("directorio esperado '%s', obtenido '%s'", tmpDir, mock.lastDir)
	}

	expectedArgs := []string{"--mode", "plan", "-i", "analizar auth"}
	if len(mock.lastArgs) != len(expectedArgs) {
		t.Fatalf("cantidad de argumentos inesperada: %v", mock.lastArgs)
	}
	for i, arg := range expectedArgs {
		if mock.lastArgs[i] != arg {
			t.Errorf("arg[%d]: esperado '%s', obtenido '%s'", i, arg, mock.lastArgs[i])
		}
	}
}

func TestChatCmd_AutonomousPermission(t *testing.T) {
	tmpDir := t.TempDir()
	originalRunner := defaultSessionRunner
	defer func() { defaultSessionRunner = originalRunner }()

	mock := &mockSessionRunner{}
	defaultSessionRunner = mock

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "--perm", "autonomous", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("chatCmd autonomous falló: %v", err)
	}

	expectedArgs := []string{"--dangerously-skip-permissions", "-i", session.DefaultOrchestratorPrompt}
	if len(mock.lastArgs) != len(expectedArgs) {
		t.Fatalf("cantidad de argumentos inesperada: %v", mock.lastArgs)
	}
	for i, arg := range expectedArgs {
		if mock.lastArgs[i] != arg {
			t.Errorf("arg[%d]: esperado '%s', obtenido '%s'", i, arg, mock.lastArgs[i])
		}
	}
}

func TestChatCmd_ErrorPropagation(t *testing.T) {
	tmpDir := t.TempDir()
	originalRunner := defaultSessionRunner
	defer func() { defaultSessionRunner = originalRunner }()

	mock := &mockSessionRunner{returnErr: errors.New("falla al ejecutar agy")}
	defaultSessionRunner = mock

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "--dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("se esperaba error y se obtuvo nil")
	}
	if !strings.Contains(err.Error(), "falla al ejecutar agy") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestChatCmd_Provider_Unavailable(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "--provider", "claude", "--dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("se esperaba error con proveedor no instalado (claude) y se obtuvo nil")
	}
	if !strings.Contains(err.Error(), "no está instalado en tu sistema") {
		t.Errorf("error esperado indicando no instalado, obtenido: %v", err)
	}
	if !strings.Contains(err.Error(), "npm install -g @anthropic-ai/claude-code") {
		t.Errorf("error debe contener InstallHint, obtenido: %v", err)
	}
}

func TestChatCmd_Provider_Unknown(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "--provider", "fantasma", "--dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("se esperaba error con proveedor desconocido y se obtuvo nil")
	}
	if !strings.Contains(err.Error(), "proveedor inválido 'fantasma'") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestChatCmd_Provider_ShorthandAndPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	originalRunner := defaultSessionRunner
	defer func() { defaultSessionRunner = originalRunner }()

	mock := &mockSessionRunner{}
	defaultSessionRunner = mock

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "-p", "agy", "-i", "prompt shorthand", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("chatCmd con -p agy -i falló: %v", err)
	}

	if mock.lastBinary != "agy" {
		t.Errorf("binario esperado 'agy', obtenido '%s'", mock.lastBinary)
	}

	foundPrompt := false
	for idx, arg := range mock.lastArgs {
		if arg == "-i" && idx+1 < len(mock.lastArgs) && mock.lastArgs[idx+1] == "prompt shorthand" {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Errorf("no se encontró '-i prompt shorthand' en argumentos: %v", mock.lastArgs)
	}
}

func TestSessionCmd_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	oldStore := defaultSessionStore
	defaultSessionStore = session.NewFileStore(tmpDir)
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "list", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session list falló: %v", err)
	}

	if !strings.Contains(buf.String(), "No hay sesiones registradas") {
		t.Errorf("salida inesperada: %s", buf.String())
	}
}

func TestSessionCmd_List_WithSessions(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:              "abc12345",
		PID:             999,
		Status:          session.StatusRunning,
		PermissionLevel: session.PermissionSupervised,
		WorkingDir:      tmpDir,
		StartedAt:       time.Now(),
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "list", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session list falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "abc12345") || !strings.Contains(out, "running") {
		t.Errorf("la salida de list no contiene la sesión: %s", out)
	}
}

func TestSessionCmd_Kill_Success(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:              "killme01",
		PID:             7777,
		Status:          session.StatusRunning,
		PermissionLevel: session.PermissionSupervised,
		WorkingDir:      tmpDir,
		StartedAt:       time.Now(),
	})

	oldStore := defaultSessionStore
	oldKiller := defaultSessionKiller
	defaultSessionStore = store
	mockK := &mockKiller{}
	defaultSessionKiller = mockK
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionKiller = oldKiller
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "kill", "killme01", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session kill falló: %v", err)
	}

	if mockK.lastPID != 7777 {
		t.Errorf("PID enviado al killer %d, esperado 7777", mockK.lastPID)
	}
	if !strings.Contains(buf.String(), "killme01") || !strings.Contains(buf.String(), "finalizada con éxito") {
		t.Errorf("mensaje de confirmación inesperado: %s", buf.String())
	}
}

func TestSessionCmd_Show(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:              "show0001",
		PID:             123,
		Status:          session.StatusCompleted,
		PermissionLevel: session.PermissionAutonomous,
		WorkingDir:      tmpDir,
		InitialPrompt:   "hola prueba",
		StartedAt:       time.Now(),
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "show", "show0001", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session show falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "show0001") || !strings.Contains(out, "completed") || !strings.Contains(out, "hola prueba") {
		t.Errorf("salida inesperada en show: %s", out)
	}
}

func TestSessionCmd_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "del_cli_01",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		StartedAt:  time.Now(),
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "delete", "del_cli_01", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session delete falló: %v", err)
	}

	if !strings.Contains(buf.String(), "del_cli_01") || !strings.Contains(buf.String(), "eliminada") {
		t.Errorf("mensaje de confirmación inesperado: %s", buf.String())
	}

	// Verificar que no existe en el store
	_, err = store.Get("del_cli_01")
	if err == nil {
		t.Error("la sesión debió haber sido eliminada del store")
	}
}

func TestSessionCmd_Resume(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:              "resume_cli_01",
		Status:          session.StatusCompleted,
		PermissionLevel: session.PermissionAutonomous,
		WorkingDir:      tmpDir,
		StartedAt:       time.Now(),
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	mockRunner := &mockSessionRunner{}
	oldRunner := defaultSessionRunner
	defaultSessionRunner = mockRunner
	defer func() { defaultSessionRunner = oldRunner }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "resume", "resume_cli_01", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session resume falló: %v", err)
	}

	// agy debe recibir --continue
	hasContinue := false
	for _, arg := range mockRunner.lastArgs {
		if arg == "--continue" {
			hasContinue = true
			break
		}
	}
	if !hasContinue {
		t.Errorf("se esperaba '--continue' en los argumentos de resume, obtenidos: %v", mockRunner.lastArgs)
	}
}

type mockCLIWorkspaceProvider struct {
	cleanedWT    string
	cleanedBr    string
	resolvedRoot string
	diffReturn   string
	diffErr      error
	mergeReturn  *workspace.MergeResult
	mergeErr     error
}

func (m *mockCLIWorkspaceProvider) IsGitAvailable(ctx context.Context, dir string) bool {
	return true
}

func (m *mockCLIWorkspaceProvider) ResolveProjectRoot(ctx context.Context, dir string) string {
	if m.resolvedRoot != "" {
		return m.resolvedRoot
	}
	return dir
}

func (m *mockCLIWorkspaceProvider) Prepare(ctx context.Context, sessionID string, baseDir string) (*workspace.Workspace, error) {
	return &workspace.Workspace{
		WorkingDir: baseDir,
		TargetDir:  baseDir,
	}, nil
}

func (m *mockCLIWorkspaceProvider) Cleanup(ctx context.Context, ws *workspace.Workspace) error {
	return nil
}

func (m *mockCLIWorkspaceProvider) CleanupWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string) error {
	m.cleanedWT = worktreeDir
	m.cleanedBr = branchName
	return nil
}

func (m *mockCLIWorkspaceProvider) DiffWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string, statOnly bool) (string, error) {
	return m.diffReturn, m.diffErr
}

func (m *mockCLIWorkspaceProvider) MergeWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string, squash bool, noCommit bool) (*workspace.MergeResult, error) {
	return m.mergeReturn, m.mergeErr
}

func (m *mockCLIWorkspaceProvider) Prune(ctx context.Context, baseDir string, activeSessionIDs []string) (*workspace.PruneReport, error) {
	return &workspace.PruneReport{}, nil
}

func TestSessionCmd_List_WorktreeAndDirect(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:              "wt_sess",
		PID:             111,
		Status:          session.StatusRunning,
		PermissionLevel: session.PermissionSupervised,
		WorkingDir:      tmpDir,
		IsIsolated:      true,
		WorktreeDir:     tmpDir + "/.harness/worktrees/wt_sess",
		BranchName:      "harness/wt_sess",
		StartedAt:       time.Now(),
	})
	_ = store.Save(&session.SessionRecord{
		ID:              "direct_sess",
		PID:             222,
		Status:          session.StatusCompleted,
		PermissionLevel: session.PermissionReadOnly,
		WorkingDir:      tmpDir,
		IsIsolated:      false,
		StartedAt:       time.Now().Add(-10 * time.Minute),
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "list", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session list falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "MODO") {
		t.Errorf("la cabecera no contiene 'MODO': %s", out)
	}
	if !strings.Contains(out, "Worktree") {
		t.Errorf("la salida no contiene 'Worktree': %s", out)
	}
	if !strings.Contains(out, "Directo") {
		t.Errorf("la salida no contiene 'Directo': %s", out)
	}
}

func TestSessionCmd_Show_Worktree(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	wtDir := tmpDir + "/.harness/worktrees/show_wt"
	_ = store.Save(&session.SessionRecord{
		ID:              "show_wt",
		PID:             456,
		Status:          session.StatusRunning,
		PermissionLevel: session.PermissionAutonomous,
		WorkingDir:      tmpDir,
		IsIsolated:      true,
		WorktreeDir:     wtDir,
		BranchName:      "harness/show_wt",
		StartedAt:       time.Now(),
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "show", "show_wt", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session show falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Worktree aislado") {
		t.Errorf("show no muestra 'Worktree aislado': %s", out)
	}
	if !strings.Contains(out, wtDir) {
		t.Errorf("show no muestra la ruta del worktree '%s': %s", wtDir, out)
	}
	if !strings.Contains(out, "harness/show_wt") {
		t.Errorf("show no muestra la rama 'harness/show_wt': %s", out)
	}
}

func TestSessionCmd_Delete_WithWorktree(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	wtDir := tmpDir + "/.harness/worktrees/del_wt_sess"
	_ = store.Save(&session.SessionRecord{
		ID:          "del_wt_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: wtDir,
		BranchName:  "harness/del_wt_sess",
		StartedAt:   time.Now(),
	})

	mockWS := &mockCLIWorkspaceProvider{}
	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "delete", "del_wt_sess", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session delete falló: %v", err)
	}

	if mockWS.cleanedWT != wtDir {
		t.Errorf("CleanupWorktree no recibió wtDir '%s', recibió '%s'", wtDir, mockWS.cleanedWT)
	}
	if mockWS.cleanedBr != "harness/del_wt_sess" {
		t.Errorf("CleanupWorktree no recibió rama 'harness/del_wt_sess', recibió '%s'", mockWS.cleanedBr)
	}
}

func TestChatCmd_ResolvesProjectRoot(t *testing.T) {
	tmpRoot := t.TempDir()
	subDir := tmpRoot + "/internal/sub"

	originalRunner := defaultSessionRunner
	originalWS := defaultSessionWorkspace
	originalStore := defaultSessionStore
	defer func() {
		defaultSessionRunner = originalRunner
		defaultSessionWorkspace = originalWS
		defaultSessionStore = originalStore
	}()

	mockRunner := &mockSessionRunner{}
	defaultSessionRunner = mockRunner

	mockWS := &mockCLIWorkspaceProvider{resolvedRoot: tmpRoot}
	defaultSessionWorkspace = mockWS

	mockStore := session.NewFileStore(tmpRoot)
	defaultSessionStore = mockStore

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"chat", "--dir", subDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("chatCmd falló: %v", err)
	}

	if mockRunner.lastDir != tmpRoot {
		t.Errorf("chat no resolvió a la raíz del proyecto: obtenido %s, esperado %s", mockRunner.lastDir, tmpRoot)
	}
}

func TestSessionCmd_List_ResolvesProjectRoot(t *testing.T) {
	tmpRoot := t.TempDir()
	subDir := tmpRoot + "/pkg/sub"

	mockWS := &mockCLIWorkspaceProvider{resolvedRoot: tmpRoot}
	mockStore := session.NewFileStore(tmpRoot)
	_ = mockStore.Save(&session.SessionRecord{
		ID:         "root_sess_01",
		Status:     session.StatusCompleted,
		WorkingDir: tmpRoot,
		StartedAt:  time.Now(),
	})

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = mockStore
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "list", "--dir", subDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session list falló: %v", err)
	}

	if !strings.Contains(buf.String(), "root_sess_01") {
		t.Errorf("session list en subdirectorio no encontró la sesión en la raíz del proyecto: %s", buf.String())
	}
}

func TestSessionCmd_Path_IsolatedAndDirect(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)

	wtPath := filepath.Join(tmpDir, ".harness", "worktrees", "path_cli_iso")
	_ = store.Save(&session.SessionRecord{
		ID:          "path_cli_iso",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: wtPath,
	})
	_ = store.Save(&session.SessionRecord{
		ID:         "path_cli_dir",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	// 1. Isolated
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"session", "path", "path_cli_iso", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session path falló: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != filepath.Clean(wtPath) {
		t.Errorf("session path aislado esperado '%s', obtenido '%s'", filepath.Clean(wtPath), out)
	}

	// 2. Direct
	cmd = NewRootCmd()
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"session", "path", "path_cli_dir", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("session path falló: %v", err)
	}
	out = strings.TrimSpace(buf.String())
	if out != filepath.Clean(tmpDir) {
		t.Errorf("session path directo esperado '%s', obtenido '%s'", filepath.Clean(tmpDir), out)
	}
}

func TestSessionCmd_Path_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	bufErr := new(bytes.Buffer)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(bufErr)
	cmd.SetArgs([]string{"session", "path", "no_exist", "--dir", tmpDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("session path inexistente debió retornar error")
	}
}

func TestSessionCmd_Diff_Direct(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "diff_dir_sess",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "diff", "diff_dir_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session diff falló: %v", err)
	}
	if !strings.Contains(buf.String(), "modo directo") {
		t.Errorf("salida esperada que mencione modo directo, obtenida: %s", buf.String())
	}
}

func TestSessionCmd_Diff_Isolated_FullAndStat(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "diff_iso_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/diff_iso_sess",
		BranchName:  "harness/diff_iso_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		diffReturn: "diff --git a/test.go b/test.go\n+new line",
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	// Diff completo
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "diff", "diff_iso_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session diff falló: %v", err)
	}
	if !strings.Contains(buf.String(), "+new line") {
		t.Errorf("diff no contiene el contenido esperado: %s", buf.String())
	}

	// Diff con --stat
	mockWS.diffReturn = "test.go | 1 +\n 1 file changed, 1 insertion(+)"
	cmd = NewRootCmd()
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "diff", "diff_iso_sess", "--stat", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("session diff --stat falló: %v", err)
	}
	if !strings.Contains(buf.String(), "test.go | 1 +") {
		t.Errorf("diff --stat no contiene estadísticas: %s", buf.String())
	}
}

func TestSessionCmd_Diff_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "diff_empty_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/diff_empty_sess",
		BranchName:  "harness/diff_empty_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		diffReturn: "",
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "diff", "diff_empty_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session diff falló: %v", err)
	}
	if !strings.Contains(buf.String(), "No hay cambios") {
		t.Errorf("salida esperada de no hay cambios: %s", buf.String())
	}
}

func TestSessionCmd_Merge_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "merge_cli_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/merge_cli_sess",
		BranchName:  "harness/merge_cli_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		mergeReturn: &workspace.MergeResult{
			Message:         "merge ok",
			FilesIntegrated: []string{"main.go", "util.go"},
			AlreadyUpToDate: false,
		},
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "merge", "merge_cli_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session merge falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fusión completada con éxito") {
		t.Errorf("merge no muestra mensaje de éxito: %s", out)
	}
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "util.go") {
		t.Errorf("merge no lista archivos integrados: %s", out)
	}
}

func TestSessionCmd_Merge_SquashAndNoCommit(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "merge_flags_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/merge_flags_sess",
		BranchName:  "harness/merge_flags_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		mergeReturn: &workspace.MergeResult{
			Message:         "squash merge ok",
			FilesIntegrated: []string{"flagged.go"},
			AlreadyUpToDate: false,
		},
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "merge", "merge_flags_sess", "--squash", "--no-commit", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session merge falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fusión completada con éxito") {
		t.Errorf("mensaje de éxito no presente: %s", out)
	}
	if !strings.Contains(out, "stage sin comitear") {
		t.Errorf("nota de stage no-commit no presente: %s", out)
	}
}

func TestSessionCmd_Merge_DirectError(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "merge_dir_err",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"session", "merge", "merge_dir_err", "--dir", tmpDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("session merge en sesión directa debió fallar")
	}
}

func TestSessionCmd_Merge_AlreadyUpToDate(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "merge_uptodate_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/merge_uptodate_sess",
		BranchName:  "harness/merge_uptodate_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		mergeReturn: &workspace.MergeResult{
			Message:         "already up to date",
			FilesIntegrated: nil,
			AlreadyUpToDate: true,
		},
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "merge", "merge_uptodate_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session merge falló: %v", err)
	}

	if !strings.Contains(buf.String(), "ya está actualizada") {
		t.Errorf("salida esperada 'ya está actualizada': %s", buf.String())
	}
}

func TestSessionCmd_Read_Direct(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "read_dir_sess",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "read", "read_dir_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session read falló: %v", err)
	}
	if !strings.Contains(buf.String(), "modo directo") {
		t.Errorf("salida esperada que mencione modo directo, obtenida: %s", buf.String())
	}
}

func TestSessionCmd_Read_Isolated_FullAndStat(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "read_iso_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/read_iso_sess",
		BranchName:  "harness/read_iso_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		diffReturn: "diff --git a/test.go b/test.go\n+new line",
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	// Read completo
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "read", "read_iso_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session read falló: %v", err)
	}
	if !strings.Contains(buf.String(), "+new line") {
		t.Errorf("read no contiene el contenido esperado: %s", buf.String())
	}

	// Read con --stat
	mockWS.diffReturn = "test.go | 1 +\n 1 file changed, 1 insertion(+)"
	cmd = NewRootCmd()
	buf = new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "read", "read_iso_sess", "--stat", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("session read --stat falló: %v", err)
	}
	if !strings.Contains(buf.String(), "test.go | 1 +") {
		t.Errorf("read --stat no contiene estadísticas: %s", buf.String())
	}
}

func TestSessionCmd_Read_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "read_empty_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/read_empty_sess",
		BranchName:  "harness/read_empty_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		diffReturn: "",
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "read", "read_empty_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session read falló: %v", err)
	}
	if !strings.Contains(buf.String(), "No hay cambios") {
		t.Errorf("salida esperada de no hay cambios: %s", buf.String())
	}
}

func TestSessionCmd_Get_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "get_cli_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/get_cli_sess",
		BranchName:  "harness/get_cli_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		mergeReturn: &workspace.MergeResult{
			Message:         "merge ok",
			FilesIntegrated: []string{"main.go", "util.go"},
			AlreadyUpToDate: false,
		},
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "get", "get_cli_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session get falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Cambios del worktree traídos e integrados con éxito") {
		t.Errorf("get no muestra mensaje de éxito: %s", out)
	}
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "util.go") {
		t.Errorf("get no lista archivos integrados: %s", out)
	}
}

func TestSessionCmd_Get_SquashAndNoCommit(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "get_flags_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/get_flags_sess",
		BranchName:  "harness/get_flags_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		mergeReturn: &workspace.MergeResult{
			Message:         "squash merge ok",
			FilesIntegrated: []string{"flagged.go"},
			AlreadyUpToDate: false,
		},
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "get", "get_flags_sess", "--squash", "--no-commit", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session get falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Cambios del worktree traídos e integrados con éxito") {
		t.Errorf("mensaje de éxito no presente: %s", out)
	}
	if !strings.Contains(out, "stage sin comitear") {
		t.Errorf("nota de stage no-commit no presente: %s", out)
	}
}

func TestSessionCmd_Get_DirectError(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "get_dir_err",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		IsIsolated: false,
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"session", "get", "get_dir_err", "--dir", tmpDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("session get en sesión directa debió fallar")
	}
}

func TestSessionCmd_Get_AlreadyUpToDate(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:          "get_uptodate_sess",
		Status:      session.StatusCompleted,
		WorkingDir:  tmpDir,
		IsIsolated:  true,
		WorktreeDir: tmpDir + "/.harness/worktrees/get_uptodate_sess",
		BranchName:  "harness/get_uptodate_sess",
	})

	mockWS := &mockCLIWorkspaceProvider{
		mergeReturn: &workspace.MergeResult{
			Message:         "already up to date",
			FilesIntegrated: nil,
			AlreadyUpToDate: true,
		},
	}

	oldStore := defaultSessionStore
	oldWS := defaultSessionWorkspace
	defaultSessionStore = store
	defaultSessionWorkspace = mockWS
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionWorkspace = oldWS
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "get", "get_uptodate_sess", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session get falló: %v", err)
	}

	if !strings.Contains(buf.String(), "ya está actualizada") {
		t.Errorf("salida esperada 'ya está actualizada': %s", buf.String())
	}
}

type mockCLIMetricsService struct {
	lastSessionID string
	lastWorkDir   string
	returnMetrics *metrics.SessionMetrics
	returnErr     error
}

func (m *mockCLIMetricsService) GetMetrics(ctx context.Context, sessionID string, workDir string) (*metrics.SessionMetrics, error) {
	m.lastSessionID = sessionID
	m.lastWorkDir = workDir
	return m.returnMetrics, m.returnErr
}

func sampleTestMetrics() *metrics.SessionMetrics {
	started := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
	finished := time.Date(2026, 9, 24, 18, 5, 0, 0, time.UTC)
	subStarted := time.Date(2026, 9, 24, 18, 1, 0, 0, time.UTC)
	subFinished := time.Date(2026, 9, 24, 18, 2, 30, 0, time.UTC)

	return &metrics.SessionMetrics{
		SessionID:      "sess-cli-metrics",
		ConversationID: "conv-parent-001",
		TotalDuration:  5 * time.Minute,
		StartedAt:      started,
		FinishedAt:     &finished,
		StepsCount:     24,
		Tokens: metrics.TokenUsage{
			PromptTokens:     1500,
			ThinkingTokens:   800,
			CompletionTokens: 600,
			TotalTokens:      2900,
		},
		ToolCalls: map[string]int{
			"run_command":          10,
			"replace_file_content": 4,
			"view_file":            8,
			"invoke_subagent":      1,
		},
		Subagents: []metrics.SubagentMetrics{
			{
				ConversationID: "conv-sub-001",
				Role:           "Core Engineer",
				TypeName:       "self",
				Prompt:         "Implementar feature completa",
				Duration:       90 * time.Second,
				StartedAt:      subStarted,
				FinishedAt:     &subFinished,
				StepsCount:     12,
				Tokens: metrics.TokenUsage{
					PromptTokens:     700,
					ThinkingTokens:   300,
					CompletionTokens: 400,
					TotalTokens:      1400,
				},
				ToolCalls: map[string]int{
					"run_command": 5,
				},
				Status: "completed",
			},
		},
	}
}

func TestSessionCmd_Metrics_Formatted(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "sess-cli-metrics",
		WorkingDir: tmpDir,
	})

	mockMetrics := &mockCLIMetricsService{
		returnMetrics: sampleTestMetrics(),
	}

	oldStore := defaultSessionStore
	oldMetrics := defaultSessionMetrics
	defaultSessionStore = store
	defaultSessionMetrics = mockMetrics
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionMetrics = oldMetrics
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "metrics", "sess-cli-metrics", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session metrics falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Telemetría y Métricas de Sesión: sess-cli-metrics") {
		t.Errorf("falta título en salida: %s", out)
	}
	if !strings.Contains(out, "conv-parent-001") {
		t.Errorf("falta conversación en salida: %s", out)
	}
	if !strings.Contains(out, "2900 tokens") {
		t.Errorf("falta total de tokens en salida: %s", out)
	}
	if !strings.Contains(out, "run_command:") || !strings.Contains(out, "10 llamadas") {
		t.Errorf("falta resumen de herramientas en salida: %s", out)
	}
	if !strings.Contains(out, "Core Engineer") || !strings.Contains(out, "conv-sub-001") {
		t.Errorf("falta subagente en salida: %s", out)
	}
}

func TestSessionCmd_Metrics_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "sess-cli-metrics",
		WorkingDir: tmpDir,
	})

	mockMetrics := &mockCLIMetricsService{
		returnMetrics: sampleTestMetrics(),
	}

	oldStore := defaultSessionStore
	oldMetrics := defaultSessionMetrics
	defaultSessionStore = store
	defaultSessionMetrics = mockMetrics
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionMetrics = oldMetrics
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "metrics", "sess-cli-metrics", "--dir", tmpDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session metrics --json falló: %v", err)
	}

	var parsed metrics.SessionMetrics
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("salida no es JSON válido: %v\nSalida: %s", err, buf.String())
	}

	if parsed.SessionID != "sess-cli-metrics" {
		t.Errorf("SessionID inesperado en JSON: %s", parsed.SessionID)
	}
	if parsed.Tokens.TotalTokens != 2900 {
		t.Errorf("TotalTokens inesperado en JSON: %d", parsed.Tokens.TotalTokens)
	}
	if len(parsed.Subagents) != 1 || parsed.Subagents[0].Role != "Core Engineer" {
		t.Errorf("Subagentes inesperados en JSON: %+v", parsed.Subagents)
	}
}

func TestSessionCmd_Metrics_Detailed(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "sess-cli-metrics",
		WorkingDir: tmpDir,
	})

	mockMetrics := &mockCLIMetricsService{
		returnMetrics: sampleTestMetrics(),
	}

	oldStore := defaultSessionStore
	oldMetrics := defaultSessionMetrics
	defaultSessionStore = store
	defaultSessionMetrics = mockMetrics
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionMetrics = oldMetrics
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "metrics", "sess-cli-metrics", "--dir", tmpDir, "--detailed"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session metrics --detailed falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Prompt: Implementar feature completa") {
		t.Errorf("falta prompt detallado del subagente: %s", out)
	}
	if !strings.Contains(out, "Inicio:") || !strings.Contains(out, "Fin:") {
		t.Errorf("faltan marcas de tiempo de subagente en salida detallada: %s", out)
	}
}

func TestSessionCmd_Metrics_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store := session.NewFileStore(tmpDir)

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "metrics", "non-existent-id", "--dir", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("se esperaba error para sesión inexistente")
	}
}

func TestSessionCmd_Log_Success(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "log", "sess-test-log", "-a", "Explorando repositorio", "-s", "READ", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session log falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Evento registrado exitosamente para la sesión 'sess-test-log'") {
		t.Errorf("salida inesperada: %s", out)
	}

	// Verificar archivo de eventos
	loggerSvc := logger.NewService(tmpDir)
	events, err := loggerSvc.GetEvents(context.Background(), "sess-test-log")
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("se esperaba 1 evento, obtenidos %d", len(events))
	}
	if events[0].Stage != logger.StageRead || events[0].Action != "Explorando repositorio" || events[0].Status != nil {
		t.Errorf("evento no coincide: %+v", events[0])
	}
}

func TestSessionCmd_Log_WithStatusAndDetails(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"session", "log", "sess-full-log",
		"-a", "Tarea terminada",
		"-s", "FINISH",
		"--status", "OK",
		"--agent", "subagent-1",
		"-r", "implementer",
		"--duration", "1500",
		"--dir", tmpDir,
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session log falló: %v", err)
	}

	loggerSvc := logger.NewService(tmpDir)
	events, err := loggerSvc.GetEvents(context.Background(), "sess-full-log")
	if err != nil {
		t.Fatalf("GetEvents falló: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("se esperaba 1 evento, obtenidos %d", len(events))
	}

	e := events[0]
	if e.Stage != logger.StageFinish || e.Status == nil || *e.Status != logger.StatusOK {
		t.Errorf("stage o status incorrectos: %+v", e)
	}
	if e.AgentID != "subagent-1" || e.Role != "implementer" {
		t.Errorf("agent o role incorrectos: %+v", e)
	}
	if e.DurationMs == nil || *e.DurationMs != 1500 {
		t.Errorf("durationMs incorrecto: %+v", e.DurationMs)
	}
}

func TestSessionCmd_Log_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Falta argumento action
	cmd1 := NewRootCmd()
	buf1 := new(bytes.Buffer)
	cmd1.SetOut(buf1)
	cmd1.SetErr(buf1)
	cmd1.SetArgs([]string{"session", "log", "s1", "-s", "READ", "--dir", tmpDir})
	if err := cmd1.Execute(); err == nil {
		t.Error("se esperaba error por falta de --action")
	}

	// 2. Falta argumento stage
	cmd2 := NewRootCmd()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"session", "log", "s1", "-a", "algo", "--dir", tmpDir})
	if err := cmd2.Execute(); err == nil {
		t.Error("se esperaba error por falta de --stage")
	}

	// 3. Stage inválido
	cmd3 := NewRootCmd()
	buf3 := new(bytes.Buffer)
	cmd3.SetOut(buf3)
	cmd3.SetErr(buf3)
	cmd3.SetArgs([]string{"session", "log", "s1", "-a", "algo", "-s", "INVALID_STAGE", "--dir", tmpDir})
	if err := cmd3.Execute(); err == nil {
		t.Error("se esperaba error por stage inválido")
	}

	// 4. Status inválido
	cmd4 := NewRootCmd()
	buf4 := new(bytes.Buffer)
	cmd4.SetOut(buf4)
	cmd4.SetErr(buf4)
	cmd4.SetArgs([]string{"session", "log", "s1", "-a", "algo", "-s", "FINISH", "--status", "INVALID_STATUS", "--dir", tmpDir})
	if err := cmd4.Execute(); err == nil {
		t.Error("se esperaba error por status inválido")
	}
}

func TestSessionCmd_Logs_EmptyAndFormatted(t *testing.T) {
	tmpDir := t.TempDir()
	sessID := "sess-logs-view"

	// 1. Logs vacío
	cmdEmpty := NewRootCmd()
	bufEmpty := new(bytes.Buffer)
	cmdEmpty.SetOut(bufEmpty)
	cmdEmpty.SetErr(bufEmpty)
	cmdEmpty.SetArgs([]string{"session", "logs", sessID, "--dir", tmpDir})
	if err := cmdEmpty.Execute(); err != nil {
		t.Fatalf("session logs vacío falló: %v", err)
	}
	if !strings.Contains(bufEmpty.String(), "No hay eventos registrados") {
		t.Errorf("salida esperada para log vacío no encontrada: %s", bufEmpty.String())
	}

	// 2. Registrar eventos
	loggerSvc := logger.NewService(tmpDir)
	_ = loggerSvc.Emit(context.Background(), &logger.Event{
		SessionID: sessID,
		AgentID:   "orch",
		Role:      "orchestrator",
		Action:    "Iniciando análisis",
		Stage:     logger.StageRead,
		Status:    nil,
	})
	_ = loggerSvc.Emit(context.Background(), &logger.Event{
		SessionID: sessID,
		AgentID:   "orch",
		Role:      "orchestrator",
		Action:    "Finalizando tarea",
		Stage:     logger.StageFinish,
		Status:    logger.StatusPtr(logger.StatusOK),
	})

	// 3. Consultar logs formateados
	cmdLogs := NewRootCmd()
	bufLogs := new(bytes.Buffer)
	cmdLogs.SetOut(bufLogs)
	cmdLogs.SetErr(bufLogs)
	cmdLogs.SetArgs([]string{"session", "logs", sessID, "--dir", tmpDir})
	if err := cmdLogs.Execute(); err != nil {
		t.Fatalf("session logs falló: %v", err)
	}

	out := bufLogs.String()
	if !strings.Contains(out, "[READ]") || !strings.Contains(out, "[FINISH]") {
		t.Errorf("falta etapa en salida de logs: %s", out)
	}
	if !strings.Contains(out, "(status: null)") || !strings.Contains(out, "(status: OK)") {
		t.Errorf("falta estado en salida de logs: %s", out)
	}
	if !strings.Contains(out, "Iniciando análisis") || !strings.Contains(out, "Finalizando tarea") {
		t.Errorf("falta descripción de acción en salida de logs: %s", out)
	}
}

func TestSessionCmd_Logs_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	sessID := "sess-logs-json"

	loggerSvc := logger.NewService(tmpDir)
	_ = loggerSvc.Emit(context.Background(), &logger.Event{
		SessionID: sessID,
		AgentID:   "agent-json",
		Role:      "worker",
		Action:    "Paso 1 json",
		Stage:     logger.StagePending,
	})

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "logs", sessID, "--json", "--dir", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("session logs --json falló: %v", err)
	}

	line := strings.TrimSpace(buf.String())
	var evt logger.Event
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		t.Fatalf("la salida no es un JSON válido de Event: %v (salida: %s)", err, line)
	}
	if evt.SessionID != sessID || evt.Action != "Paso 1 json" {
		t.Errorf("datos deserializados no coinciden: %+v", evt)
	}
}

func TestSessionCmd_Logs_Follow(t *testing.T) {
	tmpDir := t.TempDir()
	sessID := "sess-logs-follow"

	loggerSvc := logger.NewService(tmpDir)
	_ = loggerSvc.Emit(context.Background(), &logger.Event{
		SessionID: sessID,
		Action:    "Evento inicial",
		Stage:     logger.StageRead,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "logs", sessID, "-f", "--dir", tmpDir})

	// Ejecutar con contexto con timeout (simula interrupción del follow)
	_ = cmd.ExecuteContext(ctx)

	out := buf.String()
	if !strings.Contains(out, "Evento inicial") || !strings.Contains(out, "[READ]") {
		t.Errorf("salida esperada de follow no encontrada: %s", out)
	}
}

type mockUpdaterService struct {
	checkInfo   *updater.ReleaseInfo
	checkErr    error
	updateRes   *updater.UpdateResult
	updateErr   error
	lastVer     string
	lastInstDir string
}

func (m *mockUpdaterService) CheckLatest(ctx context.Context) (*updater.ReleaseInfo, error) {
	return m.checkInfo, m.checkErr
}

func (m *mockUpdaterService) Update(ctx context.Context, targetVer string, installDir string) (*updater.UpdateResult, error) {
	m.lastVer = targetVer
	m.lastInstDir = installDir
	return m.updateRes, m.updateErr
}

func TestUpdateCmd_Check_AlreadyUpToDate(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkInfo: &updater.ReleaseInfo{
			Version: "0.0.1",
			IsNewer: false,
		},
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update", "--check"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("update --check falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ya se encuentra en la versión más reciente") {
		t.Errorf("salida esperada de versión actualizada no encontrada: %s", out)
	}
}

func TestUpdateCmd_Check_NewerVersion(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkInfo: &updater.ReleaseInfo{
			Version: "1.2.0",
			IsNewer: true,
		},
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update", "--check"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("update --check falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Hay una nueva versión disponible (v1.2.0)") {
		t.Errorf("salida esperada de nueva versión no encontrada: %s", out)
	}
}

func TestUpdateCmd_Check_Error(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkErr: errors.New("falla simulada de red"),
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update", "--check"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("se esperaba error y se obtuvo nil")
	}
	if !strings.Contains(err.Error(), "error al verificar actualizaciones") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestUpdateCmd_Update_AlreadyUpToDate(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkInfo: &updater.ReleaseInfo{
			Version: "0.0.1",
			IsNewer: false,
		},
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("update falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ya se encuentra en la versión más reciente") {
		t.Errorf("salida inesperada: %s", out)
	}
	if mock.lastVer != "" {
		t.Errorf("Update no debió haberse invocado, pero se invocó con versión: %s", mock.lastVer)
	}
}

func TestUpdateCmd_Update_WithForce(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkInfo: &updater.ReleaseInfo{
			Version: "0.0.1",
			IsNewer: false,
		},
		updateRes: &updater.UpdateResult{
			Version:    "0.0.1",
			InstallDir: "/test/bin",
		},
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update", "--force"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("update --force falló: %v", err)
	}

	if mock.lastVer != "0.0.1" {
		t.Errorf("se esperaba invocación de Update con '0.0.1', obtenida '%s'", mock.lastVer)
	}
	out := buf.String()
	if !strings.Contains(out, "actualizado exitosamente a v0.0.1") {
		t.Errorf("salida inesperada: %s", out)
	}
}

func TestUpdateCmd_Update_WithExplicitVersion(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		updateRes: &updater.UpdateResult{
			Version:    "2.0.0",
			InstallDir: "/custom/bin",
		},
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update", "--version", "2.0.0"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("update --version falló: %v", err)
	}

	if mock.lastVer != "2.0.0" {
		t.Errorf("se esperaba targetVer '2.0.0', obtenido '%s'", mock.lastVer)
	}
	out := buf.String()
	if !strings.Contains(out, "actualizado exitosamente a v2.0.0") {
		t.Errorf("salida inesperada: %s", out)
	}
}

func TestUpdateCmd_Update_Success(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkInfo: &updater.ReleaseInfo{
			Version: "1.5.0",
			IsNewer: true,
		},
		updateRes: &updater.UpdateResult{
			Version:    "1.5.0",
			InstallDir: "/home/user/.local/bin",
		},
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("update falló: %v", err)
	}

	if mock.lastVer != "1.5.0" {
		t.Errorf("se esperaba targetVer '1.5.0', obtenido '%s'", mock.lastVer)
	}
	out := buf.String()
	if !strings.Contains(out, "actualizado exitosamente a v1.5.0") {
		t.Errorf("salida inesperada: %s", out)
	}
}

func TestUpdateCmd_Update_Error(t *testing.T) {
	oldUpdater := defaultUpdaterService
	mock := &mockUpdaterService{
		checkInfo: &updater.ReleaseInfo{
			Version: "1.5.0",
			IsNewer: true,
		},
		updateErr: errors.New("permiso denegado"),
	}
	defaultUpdaterService = mock
	defer func() { defaultUpdaterService = oldUpdater }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"update"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("se esperaba error y se obtuvo nil")
	}
	if !strings.Contains(err.Error(), "error durante la actualización de gz-ia") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestSessionCmd_Prune(t *testing.T) {
	tmpDir := t.TempDir()

	mockWS := &mockCLIWorkspaceProvider{}
	oldWS := defaultSessionWorkspace
	defaultSessionWorkspace = mockWS
	defer func() { defaultSessionWorkspace = oldWS }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "prune", "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session prune falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "sincronizado") && !strings.Contains(out, "Reconciliación") {
		t.Errorf("salida inesperada de session prune: %s", out)
	}
}

func TestSessionCmd_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()

	// Crear sesión y manifest
	store := session.NewFileStore(tmpDir)
	sessID := "sess_cli_cleanup"
	createdRel := "temp_unproject_file.txt"
	createdPath := filepath.Join(tmpDir, createdRel)
	_ = os.WriteFile(createdPath, []byte("temp"), 0644)

	manifest := workspace.NewManifest(sessID)
	manifest.CreatedFiles = []string{createdRel}
	_ = workspace.SaveManifest(tmpDir, manifest)

	_ = store.Save(&session.SessionRecord{
		ID:         sessID,
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
	})

	oldStore := defaultSessionStore
	defaultSessionStore = store
	defer func() { defaultSessionStore = oldStore }()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"session", "cleanup", sessID, "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session cleanup falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "limpiado y desproyectado con éxito") {
		t.Errorf("salida inesperada de session cleanup: %s", out)
	}

	if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
		t.Errorf("archivo %s debió haber sido eliminado por session cleanup", createdPath)
	}
}

func TestToolkitCmd_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GZ_TOOLING_DIR", filepath.Join(tmpDir, "empty-global-tooling"))

	// 1. Toolkit list vacio
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"toolkit", "list", "--dir", tmpDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("toolkit list falló: %v", err)
	}
	if !strings.Contains(buf.String(), "No hay toolkits ni presets") {
		t.Errorf("salida inesperada: %s", buf.String())
	}

	// 2. Toolkit create (o init)
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"toolkit", "create", "frontend", "--desc", "Frontend React/Vue", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("toolkit create falló: %v", err)
	}
	if !strings.Contains(buf.String(), "creado exitosamente") {
		t.Errorf("salida inesperada en create: %s", buf.String())
	}

	// 3. Toolkit list con el toolkit creado
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"toolkit", "list", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("toolkit list falló: %v", err)
	}
	if !strings.Contains(buf.String(), "frontend") {
		t.Errorf("se esperaba 'frontend' en list: %s", buf.String())
	}

	// 4. Toolkit show
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"toolkit", "show", "frontend", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("toolkit show falló: %v", err)
	}
	if !strings.Contains(buf.String(), "AGENTS.md") {
		t.Errorf("se esperaba AGENTS.md en show: %s", buf.String())
	}

	// 5. Toolkit path
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"toolkit", "path", "frontend", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("toolkit path falló: %v", err)
	}
	if !strings.Contains(buf.String(), "frontend") {
		t.Errorf("salida de path inesperada: %s", buf.String())
	}

	// 6. Toolkit skills
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"toolkit", "skills", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("toolkit skills falló: %v", err)
	}
	if !strings.Contains(buf.String(), "example") {
		t.Errorf("se esperaba 'example' skill en skills: %s", buf.String())
	}

	// 7. Chat with -P flag
	mockRunner := &mockSessionRunner{}
	defaultSessionRunner = mockRunner
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"chat", "-P", "frontend", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("chat -P falló: %v", err)
	}

	// 8. Chat with -T flag
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"chat", "-T", "frontend", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("chat -T falló: %v", err)
	}

	// 9. Compatibilidad con alias profile list
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"profile", "list", "--dir", tmpDir})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("profile alias list falló: %v", err)
	}
	if !strings.Contains(buf.String(), "frontend") {
		t.Errorf("se esperaba 'frontend' en alias profile list: %s", buf.String())
	}
}

func TestVaultCmd_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	buf := new(bytes.Buffer)

	// 1. Path
	cmd := NewRootCmd()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "path", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault path falló: %v", err)
	}
	if !strings.Contains(buf.String(), ".harness") || !strings.Contains(buf.String(), "vault.json") {
		t.Errorf("vault path inesperado: %s", buf.String())
	}

	// 2. List (inicial con recomendaciones)
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "list", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault list falló: %v", err)
	}
	if !strings.Contains(buf.String(), "ANTHROPIC_API_KEY") {
		t.Errorf("vault list debió incluir recomendaciones: %s", buf.String())
	}

	// 3. Set
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "set", "TEST_SECRET_KEY", "super-secret-password-12345", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault set falló: %v", err)
	}
	if !strings.Contains(buf.String(), "guardada exitosamente") {
		t.Errorf("vault set salida inesperada: %s", buf.String())
	}

	// 4. Get (enmascarado por defecto)
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "get", "TEST_SECRET_KEY", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault get falló: %v", err)
	}
	if strings.Contains(buf.String(), "super-secret-password-12345") {
		t.Errorf("vault get NO debe exponer el secreto en texto plano sin --reveal: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "sup...45") {
		t.Errorf("vault get debió mostrar valor enmascarado: %s", buf.String())
	}

	// 5. Get con --reveal
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "get", "TEST_SECRET_KEY", "--reveal", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault get --reveal falló: %v", err)
	}
	if !strings.Contains(buf.String(), "super-secret-password-12345") {
		t.Errorf("vault get --reveal debió revelar el valor: %s", buf.String())
	}

	// 6. Delete
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "delete", "TEST_SECRET_KEY", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault delete falló: %v", err)
	}
	if !strings.Contains(buf.String(), "eliminada") {
		t.Errorf("vault delete salida inesperada: %s", buf.String())
	}

	// 7. Get tras eliminación
	cmd = NewRootCmd()
	buf.Reset()
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"vault", "get", "TEST_SECRET_KEY", "--dir", tmpDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vault get tras delete falló: %v", err)
	}
	if !strings.Contains(buf.String(), "no encontrada") {
		t.Errorf("se esperaba mensaje de no encontrada: %s", buf.String())
	}
}

func TestMcpCmd_Protocol(t *testing.T) {
	tmpDir := t.TempDir()
	toolingDir := filepath.Join(tmpDir, "tooling")
	tkDir := filepath.Join(toolingDir, "toolkits", "test-tk")
	_ = os.MkdirAll(tkDir, 0755)

	// Crear tools.json
	toolsJSON := `[{"name": "test_mcp_echo", "description": "Echo tool", "command": "echo '{\"status\": \"ok\"}'"}]`
	_ = os.WriteFile(filepath.Join(tkDir, "tools.json"), []byte(toolsJSON), 0644)

	// Crear config.json con un perfil
	cfgJSON := `{
		"version": 1,
		"perfiles": [
			{"name": "mcp-test-prof", "toolkits": ["test-tk"]}
		]
	}`
	_ = os.WriteFile(filepath.Join(toolingDir, "config.json"), []byte(cfgJSON), 0644)

	// JSON-RPC requests
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"

	inBuf := bytes.NewBufferString(initReq + listReq)
	outBuf := &bytes.Buffer{}

	cmd := NewRootCmd()
	cmd.SetIn(inBuf)
	cmd.SetOut(outBuf)
	cmd.SetArgs([]string{
		"mcp",
		"--dir", tmpDir,
		"--tooling", toolingDir,
		"--profile", "mcp-test-prof",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("mcp command falló: %v", err)
	}

	outStr := outBuf.String()
	if !strings.Contains(outStr, "worktree_read") {
		t.Errorf("Se esperaba que tools/list incluyera 'worktree_read', obtenido:\n%s", outStr)
	}
	if !strings.Contains(outStr, "test_mcp_echo") {
		t.Errorf("Se esperaba que tools/list incluyera 'test_mcp_echo', obtenido:\n%s", outStr)
	}
}

func TestSessionContextCmd_Help(t *testing.T) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "context", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session context --help falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "context") {
		t.Errorf("salida de ayuda no contiene 'context': %s", out)
	}
	if !strings.Contains(out, "handoff") {
		t.Errorf("salida de ayuda no contiene 'handoff': %s", out)
	}
}

func TestSessionContextCmd_Formatted(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	loggerSvc := logger.NewService(tmpDir)
	ctx := context.Background()

	sessID := "ctx-test-01"
	_ = store.Save(&session.SessionRecord{
		ID:         sessID,
		Provider:   "agy",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		Profiles:   []string{"gz-ia"},
		StartedAt:  time.Now().Add(-30 * time.Minute),
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
			"initial_prompt": "desarrollar feature context",
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
			"exit_code":  0,
			"duration_s": int64(1800),
		},
	})

	oldStore := defaultSessionStore
	oldLogger := defaultSessionLogger
	defaultSessionStore = store
	defaultSessionLogger = loggerSvc
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionLogger = oldLogger
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "context", sessID, "--dir", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session context falló: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, sessID) {
		t.Errorf("salida no contiene session ID '%s': %s", sessID, out)
	}
	if !strings.Contains(out, "agy") {
		t.Errorf("salida no contiene proveedor 'agy': %s", out)
	}
	if !strings.Contains(out, "harness/"+sessID) {
		t.Errorf("salida no contiene rama: %s", out)
	}
}

func TestSessionContextCmd_JSON(t *testing.T) {
	tmpDir := t.TempDir()

	store := session.NewFileStore(tmpDir)
	loggerSvc := logger.NewService(tmpDir)
	ctx := context.Background()

	sessID := "ctx-json-01"
	_ = store.Save(&session.SessionRecord{
		ID:         sessID,
		Provider:   "claude",
		Status:     session.StatusCompleted,
		WorkingDir: tmpDir,
		Profiles:   []string{"maintainer"},
		StartedAt:  time.Now().Add(-10 * time.Minute),
	})

	_ = loggerSvc.Emit(ctx, &logger.Event{
		SessionID: sessID,
		AgentID:   "orchestrator",
		Role:      "orchestrator",
		Action:    "Sesión iniciada",
		Stage:     logger.StagePending,
		Metadata: map[string]any{
			"provider":       "claude",
			"toolkits":       []string{"maintainer"},
			"branch":         "harness/" + sessID,
			"initial_prompt": "probar json output",
			"working_dir":    tmpDir,
		},
	})

	oldStore := defaultSessionStore
	oldLogger := defaultSessionLogger
	defaultSessionStore = store
	defaultSessionLogger = loggerSvc
	defer func() {
		defaultSessionStore = oldStore
		defaultSessionLogger = oldLogger
	}()

	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"session", "context", sessID, "--dir", tmpDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("session context --json falló: %v", err)
	}

	var sc session.SessionContext
	if err := json.Unmarshal(buf.Bytes(), &sc); err != nil {
		t.Fatalf("JSON inválido en salida: %v\nSalida: %s", err, buf.String())
	}
	if sc.SessionID != sessID {
		t.Errorf("SessionID esperado '%s', obtenido '%s'", sessID, sc.SessionID)
	}
	if sc.Provider != "claude" {
		t.Errorf("Provider esperado 'claude', obtenido '%s'", sc.Provider)
	}
}

