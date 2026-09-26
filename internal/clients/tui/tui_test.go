package tui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"gz-ia/internal/features/logger"
	"gz-ia/internal/features/metrics"
	"gz-ia/internal/features/profile"
	"gz-ia/internal/features/session"
	"gz-ia/internal/features/updater"
	"gz-ia/internal/features/vault"
	"gz-ia/internal/features/workspace"
)

type mockRunner struct {
	called bool
}

func (m *mockRunner) Run(ctx context.Context, binary string, args []string, dir string, onStart func(pid int)) (int, error) {
	m.called = true
	if onStart != nil {
		onStart(1234)
	}
	return 0, nil
}

func TestNewClient(t *testing.T) {
	client := New(nil, nil)
	if client == nil {
		t.Fatal("New() no debe retornar nil")
	}
	if client.runner == nil {
		t.Error("client.runner no debe ser nil por defecto")
	}
	if client.killer == nil {
		t.Error("client.killer no debe ser nil por defecto")
	}
}

func TestNewClientWithCustomRunner(t *testing.T) {
	mock := &mockRunner{}
	client := New(mock, nil)
	if client == nil {
		t.Fatal("New(mock) no debe retornar nil")
	}
	if client.runner != mock {
		t.Errorf("client.runner esperado %v, obtenido %v", mock, client.runner)
	}
}

type mockTUIWorkspace struct{}

func (m *mockTUIWorkspace) IsGitAvailable(ctx context.Context, dir string) bool { return false }
func (m *mockTUIWorkspace) ResolveProjectRoot(ctx context.Context, dir string) string { return dir }
func (m *mockTUIWorkspace) Prepare(ctx context.Context, id, dir string) (*workspace.Workspace, error) {
	return nil, nil
}
func (m *mockTUIWorkspace) Cleanup(ctx context.Context, ws *workspace.Workspace) error { return nil }
func (m *mockTUIWorkspace) CleanupWorktree(ctx context.Context, b, w, br string) error {
	return nil
}
func (m *mockTUIWorkspace) DiffWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string, statOnly bool) (string, error) {
	return "", nil
}
func (m *mockTUIWorkspace) MergeWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string, squash bool, noCommit bool) (*workspace.MergeResult, error) {
	return &workspace.MergeResult{AlreadyUpToDate: true}, nil
}
func (m *mockTUIWorkspace) Prune(ctx context.Context, baseDir string, activeSessionIDs []string) (*workspace.PruneReport, error) {
	return &workspace.PruneReport{}, nil
}

func TestNewClientWithWorkspace(t *testing.T) {
	client := New(nil, nil)
	wsDef := client.getWorkspace()
	if wsDef == nil {
		t.Fatal("getWorkspace por defecto no debe ser nil")
	}

	mockWS := &mockTUIWorkspace{}
	client.WithWorkspace(mockWS)
	if client.getWorkspace() != mockWS {
		t.Error("WithWorkspace no asignó el workspace provider custom")
	}
}

func TestNewClientWithService(t *testing.T) {
	client := NewWithService(nil)
	if client == nil {
		t.Fatal("NewWithService no debe retornar nil")
	}
	if client.service != nil {
		t.Error("client.service esperado nil")
	}

	tmpDir := t.TempDir()
	client = New(nil, nil)
	svc := client.getService(tmpDir)
	if svc == nil {
		t.Fatal("client.getService no debe retornar nil")
	}

	client.WithService(svc)
	if client.service != svc {
		t.Error("WithService no asignó el service correctamente")
	}
	if client.getService(tmpDir) != svc {
		t.Error("getService debería devolver el service inyectado")
	}
}

type mockUpdaterService struct {
	checkLatestFunc func(ctx context.Context) (*updater.ReleaseInfo, error)
	updateFunc      func(ctx context.Context, targetVer string, installDir string) (*updater.UpdateResult, error)
}

func (m *mockUpdaterService) CheckLatest(ctx context.Context) (*updater.ReleaseInfo, error) {
	if m.checkLatestFunc != nil {
		return m.checkLatestFunc(ctx)
	}
	return &updater.ReleaseInfo{
		CurrentVer: "1.0.0",
		Version:    "1.0.0",
		IsNewer:    false,
	}, nil
}

func (m *mockUpdaterService) Update(ctx context.Context, targetVer string, installDir string) (*updater.UpdateResult, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, targetVer, installDir)
	}
	return &updater.UpdateResult{
		Version:           targetVer,
		InstallDir:        "/tmp/test",
		InstalledBinaries: []string{"gz-ia"},
	}, nil
}

func TestClient_WithUpdater(t *testing.T) {
	client := New(nil, nil)
	defaultUpd := client.getUpdater()
	if defaultUpd == nil {
		t.Fatal("client.getUpdater() por defecto no debe ser nil")
	}

	mockUpd := &mockUpdaterService{}
	res := client.WithUpdater(mockUpd)
	if res != client {
		t.Error("WithUpdater debe devolver el propio puntero *Client")
	}
	if client.updaterService != mockUpd {
		t.Error("client.updaterService esperado mockUpd")
	}
	if client.getUpdater() != mockUpd {
		t.Error("client.getUpdater() esperado mockUpd")
	}
}

func TestClient_HandleUpdate_UpToDate(t *testing.T) {
	mockUpd := &mockUpdaterService{
		checkLatestFunc: func(ctx context.Context) (*updater.ReleaseInfo, error) {
			return &updater.ReleaseInfo{
				CurrentVer: "1.0.0",
				Version:    "1.0.0",
				IsNewer:    false,
			}, nil
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).
		WithUpdater(mockUpd).
		WithIO(strings.NewReader("\n"), &outBuf)

	err := client.handleUpdate(GetIcons())
	if err != nil {
		t.Fatalf("handleUpdate() falló inesperadamente: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "gz-ia ya se encuentra en la versión más reciente (v1.0.0)") {
		t.Errorf("Salida esperada con mensaje de estar al día, obtenido:\n%s", output)
	}
}

func TestClient_HandleUpdate_NewVersion_Confirmed(t *testing.T) {
	var updateCalled bool
	mockUpd := &mockUpdaterService{
		checkLatestFunc: func(ctx context.Context) (*updater.ReleaseInfo, error) {
			return &updater.ReleaseInfo{
				CurrentVer: "1.0.0",
				Version:    "1.2.0",
				IsNewer:    true,
			}, nil
		},
		updateFunc: func(ctx context.Context, targetVer string, installDir string) (*updater.UpdateResult, error) {
			updateCalled = true
			return &updater.UpdateResult{
				Version:           "1.2.0",
				InstallDir:        "/usr/local/bin",
				InstalledBinaries: []string{"gz-ia"},
			}, nil
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).
		WithUpdater(mockUpd).
		WithIO(strings.NewReader("y\n\n"), &outBuf)

	err := client.handleUpdate(GetIcons())
	if err != nil {
		t.Fatalf("handleUpdate() falló inesperadamente: %v", err)
	}

	if !updateCalled {
		t.Error("updater.Update() debió ser invocado tras confirmación")
	}

	output := outBuf.String()
	if !strings.Contains(output, "gz-ia actualizado exitosamente a la versión v1.2.0") {
		t.Errorf("Salida esperada con mensaje de éxito de actualización, obtenido:\n%s", output)
	}
}

func TestClient_HandleUpdate_NewVersion_Declined(t *testing.T) {
	var updateCalled bool
	mockUpd := &mockUpdaterService{
		checkLatestFunc: func(ctx context.Context) (*updater.ReleaseInfo, error) {
			return &updater.ReleaseInfo{
				CurrentVer: "1.0.0",
				Version:    "1.2.0",
				IsNewer:    true,
			}, nil
		},
		updateFunc: func(ctx context.Context, targetVer string, installDir string) (*updater.UpdateResult, error) {
			updateCalled = true
			return nil, nil
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).
		WithUpdater(mockUpd).
		WithIO(strings.NewReader("n\n\n"), &outBuf)

	err := client.handleUpdate(GetIcons())
	if err != nil {
		t.Fatalf("handleUpdate() retornó error inesperado al cancelar: %v", err)
	}

	if updateCalled {
		t.Error("updater.Update() no debió ser llamado al rechazar")
	}

	output := outBuf.String()
	if !strings.Contains(output, "Actualización cancelada") {
		t.Errorf("Salida esperada con mensaje de cancelación, obtenido:\n%s", output)
	}
}

func TestClient_HandleUpdate_CheckLatestError(t *testing.T) {
	mockUpd := &mockUpdaterService{
		checkLatestFunc: func(ctx context.Context) (*updater.ReleaseInfo, error) {
			return nil, errors.New("fallo de red en github")
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).
		WithUpdater(mockUpd).
		WithIO(strings.NewReader("\n"), &outBuf)

	err := client.handleUpdate(GetIcons())
	if err == nil {
		t.Fatal("handleUpdate() debía retornar error cuando CheckLatest falla")
	}

	output := outBuf.String()
	if !strings.Contains(output, "Error al verificar actualizaciones") {
		t.Errorf("Salida esperada con mensaje de error, obtenido:\n%s", output)
	}
}

func TestClient_HandleUpdate_UpdateError(t *testing.T) {
	mockUpd := &mockUpdaterService{
		checkLatestFunc: func(ctx context.Context) (*updater.ReleaseInfo, error) {
			return &updater.ReleaseInfo{
				CurrentVer: "1.0.0",
				Version:    "1.2.0",
				IsNewer:    true,
			}, nil
		},
		updateFunc: func(ctx context.Context, targetVer string, installDir string) (*updater.UpdateResult, error) {
			return nil, errors.New("permiso denegado en directorio de instalación")
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).
		WithUpdater(mockUpd).
		WithIO(strings.NewReader("y\n\n"), &outBuf)

	err := client.handleUpdate(GetIcons())
	if err == nil {
		t.Fatal("handleUpdate() debía retornar error cuando Update falla")
	}

	output := outBuf.String()
	if !strings.Contains(output, "Error al actualizar gz-ia") {
		t.Errorf("Salida esperada con mensaje de error, obtenido:\n%s", output)
	}
}

func TestRenderWorktreeDiffViewer(t *testing.T) {
	diff := "diff --git a/test.go b/test.go\n@@ -1,2 +1,3 @@\n-old line\n+new line\n context line"
	rendered := renderWorktreeDiffViewer("sess_viewer_test", diff, GetIcons())

	if !strings.Contains(rendered, "Worktree Diff Viewer") {
		t.Errorf("viewer missing title: %s", rendered)
	}
	if !strings.Contains(rendered, "sess_viewer_test") {
		t.Errorf("viewer missing session id: %s", rendered)
	}
	if !strings.Contains(rendered, "+new line") {
		t.Errorf("viewer missing diff addition: %s", rendered)
	}
	if !strings.Contains(rendered, "-old line") {
		t.Errorf("viewer missing diff deletion: %s", rendered)
	}
}

type mockTUISessionService struct {
	readFunc      func(ctx context.Context, id string, statOnly bool) (string, error)
	getFunc       func(ctx context.Context, id string, opts session.MergeOptions) (*workspace.MergeResult, error)
	startChatFunc func(ctx context.Context, req session.StartChatRequest) error
}

func (m *mockTUISessionService) StartChat(ctx context.Context, req session.StartChatRequest) error {
	if m.startChatFunc != nil {
		return m.startChatFunc(ctx, req)
	}
	return nil
}
func (m *mockTUISessionService) List(ctx context.Context) ([]session.SessionRecord, error) { return nil, nil }
func (m *mockTUISessionService) GetRecord(ctx context.Context, id string) (*session.SessionRecord, error) { return nil, nil }
func (m *mockTUISessionService) GetSession(ctx context.Context, id string) (*session.SessionRecord, error) { return nil, nil }
func (m *mockTUISessionService) Kill(ctx context.Context, id string) error { return nil }
func (m *mockTUISessionService) Resume(ctx context.Context, id string) error { return nil }
func (m *mockTUISessionService) Delete(ctx context.Context, id string) error { return nil }
func (m *mockTUISessionService) Path(ctx context.Context, id string) (string, error) { return "", nil }
func (m *mockTUISessionService) Diff(ctx context.Context, id string, statOnly bool) (string, error) { return "", nil }
func (m *mockTUISessionService) Merge(ctx context.Context, id string, opts session.MergeOptions) (*workspace.MergeResult, error) { return nil, nil }
func (m *mockTUISessionService) Metrics(ctx context.Context, id string) (*metrics.SessionMetrics, error) { return nil, nil }
func (m *mockTUISessionService) LogEvent(ctx context.Context, evt *logger.Event) error { return nil }
func (m *mockTUISessionService) GetEvents(ctx context.Context, id string) ([]logger.Event, error) { return nil, nil }
func (m *mockTUISessionService) WatchEvents(ctx context.Context, id string) (<-chan logger.Event, error) { return nil, nil }

func (m *mockTUISessionService) Read(ctx context.Context, id string, statOnly bool) (string, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, id, statOnly)
	}
	return "", nil
}

func (m *mockTUISessionService) Get(ctx context.Context, id string, opts session.MergeOptions) (*workspace.MergeResult, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id, opts)
	}
	return &workspace.MergeResult{AlreadyUpToDate: true}, nil
}

func (m *mockTUISessionService) Prune(ctx context.Context) (*session.PruneResult, error) {
	return &session.PruneResult{}, nil
}

func (m *mockTUISessionService) Vault() vault.Service {
	return nil
}

func TestHandleSessionDetail_Read(t *testing.T) {
	mockSvc := &mockTUISessionService{
		readFunc: func(ctx context.Context, id string, statOnly bool) (string, error) {
			return "+sample added code\n-sample removed code", nil
		},
	}

	var outBuf bytes.Buffer
	// Option 2 is 'read', followed by Enter for the viewer prompt
	client := New(nil, nil).WithIO(strings.NewReader("2\n\n"), &outBuf)

	rec := &session.SessionRecord{
		ID:         "sess_detail_read",
		Status:     session.StatusCompleted,
		WorkingDir: "/tmp/project",
		IsIsolated: true,
	}

	err := client.handleSessionDetail(rec, mockSvc, GetIcons())
	if err != nil {
		t.Fatalf("handleSessionDetail returned unexpected error: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Worktree Diff Viewer") {
		t.Errorf("expected diff viewer in output, got: %s", out)
	}
	if !strings.Contains(out, "+sample added code") {
		t.Errorf("expected added code in diff viewer, got: %s", out)
	}
}

func TestHandleSessionDetail_Get_Success(t *testing.T) {
	var getCalled bool
	mockSvc := &mockTUISessionService{
		getFunc: func(ctx context.Context, id string, opts session.MergeOptions) (*workspace.MergeResult, error) {
			getCalled = true
			return &workspace.MergeResult{
				Message:         "merged successfully",
				FilesIntegrated: []string{"app.go", "config.go"},
				AlreadyUpToDate: false,
			}, nil
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).WithIO(strings.NewReader("y\n\n"), &outBuf)

	rec := &session.SessionRecord{
		ID:         "sess_detail_get",
		Status:     session.StatusCompleted,
		WorkingDir: "/tmp/project",
		IsIsolated: true,
	}

	err := client.executeSessionAction("get", rec, mockSvc, GetIcons())
	if err != nil {
		t.Fatalf("executeSessionAction returned unexpected error: %v", err)
	}

	if !getCalled {
		t.Error("expected svc.Get to be called")
	}

	out := outBuf.String()
	if !strings.Contains(out, "integrados exitosamente") {
		t.Errorf("expected success notification, got: %s", out)
	}
	if !strings.Contains(out, "app.go") || !strings.Contains(out, "config.go") {
		t.Errorf("expected integrated files list, got: %s", out)
	}
}

func TestHandleSessionDetail_Get_Declined(t *testing.T) {
	var getCalled bool
	mockSvc := &mockTUISessionService{
		getFunc: func(ctx context.Context, id string, opts session.MergeOptions) (*workspace.MergeResult, error) {
			getCalled = true
			return &workspace.MergeResult{}, nil
		},
	}

	var outBuf bytes.Buffer
	client := New(nil, nil).WithIO(strings.NewReader("n\n\n"), &outBuf)

	rec := &session.SessionRecord{
		ID:         "sess_detail_get_declined",
		Status:     session.StatusCompleted,
		WorkingDir: "/tmp/project",
		IsIsolated: true,
	}

	err := client.executeSessionAction("get", rec, mockSvc, GetIcons())
	if err != nil {
		t.Fatalf("executeSessionAction returned unexpected error: %v", err)
	}

	if getCalled {
		t.Error("svc.Get should not be called when user cancels confirmation")
	}
}

func TestHandleNewChat_Back(t *testing.T) {
	var chatStarted bool
	mockSvc := &mockTUISessionService{
		startChatFunc: func(ctx context.Context, req session.StartChatRequest) error {
			chatStarted = true
			return nil
		},
	}

	var outBuf bytes.Buffer
	// Opción 5 corresponde a "[←] Volver al menú principal"
	client := New(nil, nil).WithService(mockSvc).WithIO(strings.NewReader("5\n"), &outBuf)

	err := client.handleNewChat(GetIcons())
	if err != nil {
		t.Fatalf("handleNewChat retornó error inesperado: %v", err)
	}

	if chatStarted {
		t.Error("StartChat no debió ser llamado al seleccionar volver")
	}
}

func TestHandleNewChat_UnavailableDriverValidation(t *testing.T) {
	mockSvc := &mockTUISessionService{}

	var outBuf bytes.Buffer
	// Opción 2 es claude (no instalado). Falla validación, luego opción 5 es volver.
	client := New(nil, nil).WithService(mockSvc).WithIO(strings.NewReader("2\n5\n"), &outBuf)

	err := client.handleNewChat(GetIcons())
	if err != nil {
		t.Fatalf("handleNewChat retornó error inesperado: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "no está instalado en tu sistema") {
		t.Errorf("la salida debe contener el mensaje de validación de error: %s", output)
	}
	if !strings.Contains(output, "npm install -g @anthropic-ai/claude-code") {
		t.Errorf("la salida debe contener la ayuda de instalación (InstallHint): %s", output)
	}
}

func TestHandleNewChat_PermissionBack(t *testing.T) {
	var chatStarted bool
	mockSvc := &mockTUISessionService{
		startChatFunc: func(ctx context.Context, req session.StartChatRequest) error {
			chatStarted = true
			return nil
		},
	}

	var outBuf bytes.Buffer
	// Opción 1: agy (listo para usar). Luego opción 4: volver al menú en el formulario de permisos.
	client := New(nil, nil).WithService(mockSvc).WithIO(strings.NewReader("1\n4\n"), &outBuf)

	err := client.handleNewChat(GetIcons())
	if err != nil {
		t.Fatalf("handleNewChat retornó error inesperado: %v", err)
	}

	if chatStarted {
		t.Error("StartChat no debió ser llamado al cancelar permisos")
	}
}

func TestHandleNewChat_SuccessLaunch(t *testing.T) {
	var capturedReq session.StartChatRequest
	var chatStarted bool
	mockSvc := &mockTUISessionService{
		startChatFunc: func(ctx context.Context, req session.StartChatRequest) error {
			chatStarted = true
			capturedReq = req
			return nil
		},
	}

	var outBuf bytes.Buffer
	// Opción 1: agy. Luego opción 2: Con autorización (PermissionSupervised).
	client := New(nil, nil).WithService(mockSvc).WithIO(strings.NewReader("1\n2\n"), &outBuf)

	err := client.handleNewChat(GetIcons())
	if err != nil {
		t.Fatalf("handleNewChat retornó error inesperado: %v", err)
	}

	if !chatStarted {
		t.Fatal("StartChat debió ser llamado al confirmar selección")
	}

	if capturedReq.Provider != "agy" {
		t.Errorf("Provider esperado 'agy', obtenido '%s'", capturedReq.Provider)
	}
	if capturedReq.PermissionLevel != session.PermissionSupervised {
		t.Errorf("PermissionLevel esperado 'supervised', obtenido '%s'", capturedReq.PermissionLevel)
	}
	if capturedReq.BinaryPath != "agy" {
		t.Errorf("BinaryPath esperado 'agy', obtenido '%s'", capturedReq.BinaryPath)
	}
}

func TestHandleStartChat_Alias(t *testing.T) {
	var outBuf bytes.Buffer
	client := New(nil, nil).WithIO(strings.NewReader("5\n"), &outBuf)

	err := client.handleStartChat(GetIcons())
	if err != nil {
		t.Fatalf("handleStartChat alias retornó error inesperado: %v", err)
	}
}

func TestHandleNewChat_SmartPreselection_NoneAvailable(t *testing.T) {
	restore := session.SetLookPathForTesting(func(file string) (string, error) {
		return "", errors.New("no instalado")
	})
	defer restore()

	var chatStarted bool
	mockSvc := &mockTUISessionService{
		startChatFunc: func(ctx context.Context, req session.StartChatRequest) error {
			chatStarted = true
			return nil
		},
	}

	var outBuf bytes.Buffer
	// Al no haber drivers disponibles, el cursor se preselecciona en 'back' (opción 5)
	// Al presionar Enter ("\n"), acepta la opción por defecto ('back') y sale sin error.
	client := New(nil, nil).WithService(mockSvc).WithIO(strings.NewReader("\n"), &outBuf)

	err := client.handleNewChat(GetIcons())
	if err != nil {
		t.Fatalf("handleNewChat retornó error inesperado: %v", err)
	}

	if chatStarted {
		t.Error("StartChat no debió ser llamado cuando ningún driver está disponible")
	}

	output := outBuf.String()
	if !strings.Contains(output, "[✗] Google Antigravity (agy) — No instalado") {
		t.Errorf("debe listar agy como no instalado: %s", output)
	}
}

func TestClient_WithProfile(t *testing.T) {
	tmpDir := t.TempDir()
	profSvc := profile.NewService(profile.WithProjectDir(tmpDir))
	client := New(nil, nil).WithProfile(profSvc)
	if client.profileService == nil {
		t.Fatal("profileService no inicializado")
	}
	if client.getProfileService(tmpDir) != profSvc {
		t.Errorf("getProfileService debió retornar profSvc inyectado")
	}
}

func TestClient_WithVault(t *testing.T) {
	tmpDir := t.TempDir()
	vSvc := vault.NewService(tmpDir)
	client := New(nil, nil).WithVault(vSvc)
	if client.vaultService == nil {
		t.Fatal("vaultService no inicializado")
	}
	if client.getVault(tmpDir) != vSvc {
		t.Errorf("getVault debió retornar vSvc inyectado")
	}
}

func TestClient_HandleVault_Back(t *testing.T) {
	tmpDir := t.TempDir()
	vSvc := vault.NewService(tmpDir)
	var out bytes.Buffer

	// Opción 4 corresponde a "[←] Volver al menú principal"
	client := New(nil, nil).
		WithVault(vSvc).
		WithIO(strings.NewReader("4\n"), &out)

	icons := GetIcons()
	err := client.handleVault(icons)
	if err != nil {
		t.Fatalf("handleVault con salida 'back' falló: %v", err)
	}
}




