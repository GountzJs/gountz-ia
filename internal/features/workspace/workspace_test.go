package workspace

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type mockGitClient struct {
	lookPathErr error
	runResponses map[string]string // key: args joined
	runErrors    map[string]error
	calls        [][]string
}

func newMockGitClient() *mockGitClient {
	return &mockGitClient{
		runResponses: make(map[string]string),
		runErrors:    make(map[string]error),
	}
}

func (m *mockGitClient) LookPath() (string, error) {
	if m.lookPathErr != nil {
		return "", m.lookPathErr
	}
	return "/usr/bin/git", nil
}

func (m *mockGitClient) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmdKey := strings.Join(args, " ")
	m.calls = append(m.calls, args)

	if err, ok := m.runErrors[cmdKey]; ok {
		return m.runResponses[cmdKey], err
	}

	if resp, ok := m.runResponses[cmdKey]; ok {
		return resp, nil
	}

	return "", nil
}

func TestGitProvider_GitNotAvailableInPath(t *testing.T) {
	mockGit := newMockGitClient()
	mockGit.lookPathErr = errors.New("git no encontrado")

	provider := NewProvider(mockGit)
	ctx := context.Background()

	avail := provider.IsGitAvailable(ctx, "/any/dir")
	if avail {
		t.Error("IsGitAvailable debió retornar false cuando git no está en PATH")
	}

	ws, err := provider.Prepare(ctx, "sess001", "/any/dir")
	if err != nil {
		t.Fatalf("Prepare debió realizar fallback sin error: %v", err)
	}

	if ws.IsIsolated {
		t.Error("ws.IsIsolated debió ser false")
	}
	if ws.TargetDir != "/any/dir" {
		t.Errorf("ws.TargetDir esperado '/any/dir', obtenido '%s'", ws.TargetDir)
	}
	if ws.WorktreeDir != "" {
		t.Errorf("ws.WorktreeDir debió estar vacío, obtenido '%s'", ws.WorktreeDir)
	}
	if ws.BranchName != "" {
		t.Errorf("ws.BranchName debió estar vacío, obtenido '%s'", ws.BranchName)
	}
}

func TestGitProvider_NotInsideGitWorkTree(t *testing.T) {
	mockGit := newMockGitClient()
	mockGit.runResponses["rev-parse --is-inside-work-tree"] = "false"
	mockGit.runErrors["rev-parse --is-inside-work-tree"] = errors.New("not a git repository")

	provider := NewProvider(mockGit)
	ctx := context.Background()

	avail := provider.IsGitAvailable(ctx, "/regular/folder")
	if avail {
		t.Error("IsGitAvailable debió retornar false para un directorio regular sin git")
	}

	ws, err := provider.Prepare(ctx, "sess002", "/regular/folder")
	if err != nil {
		t.Fatalf("Prepare con fallback falló: %v", err)
	}

	if ws.IsIsolated {
		t.Error("ws.IsIsolated debió ser false en directorio sin git")
	}
	if ws.TargetDir != "/regular/folder" {
		t.Errorf("ws.TargetDir esperado '/regular/folder', obtenido '%s'", ws.TargetDir)
	}
}

func TestGitProvider_GitDirFailure(t *testing.T) {
	mockGit := newMockGitClient()
	mockGit.runResponses["rev-parse --is-inside-work-tree"] = "true"
	mockGit.runErrors["rev-parse --git-dir"] = errors.New("cannot determine git dir")

	provider := NewProvider(mockGit)
	ctx := context.Background()

	ws, err := provider.Prepare(ctx, "sess003", "/fake/repo")
	if err != nil {
		t.Fatalf("Prepare con fallback ante git-dir error falló: %v", err)
	}
	if ws.IsIsolated {
		t.Error("ws.IsIsolated debió ser false")
	}
}

func TestGitProvider_WorktreeAddFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wt-fail-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockGit := newMockGitClient()
	mockGit.runResponses["rev-parse --is-inside-work-tree"] = "true"
	mockGit.runResponses["rev-parse --git-dir"] = filepath.Join(tmpDir, ".git")

	expectedWT := filepath.Join(tmpDir, ".harness", "worktrees", "sess_err")
	cmdAdd := "worktree add -b harness/sess_err " + expectedWT
	mockGit.runErrors[cmdAdd] = errors.New("fatal: no se pudo crear worktree")

	provider := NewProvider(mockGit)
	ctx := context.Background()

	_, err = provider.Prepare(ctx, "sess_err", tmpDir)
	if err == nil {
		t.Fatal("Prepare debió retornar error si git worktree add falla")
	}
}

func TestGitProvider_RealGitRepoLifecycle(t *testing.T) {
	// Verificar si git existe en el entorno para correr prueba real
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no está instalado en el PATH, saltando prueba real de git")
	}

	tmpDir, err := os.MkdirTemp("", "real-git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Inicializar repositorio Git real
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v: %v (%s)", args, err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	// Crear archivo inicial y commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Proyecto Principal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "Commit inicial")

	provider := NewDefaultProvider()
	ctx := context.Background()

	// 1. Verificar IsGitAvailable
	if !provider.IsGitAvailable(ctx, tmpDir) {
		t.Fatal("IsGitAvailable debió retornar true para repo git inicializado")
	}

	// 2. Preparar sesión con Worktree aislado
	sessionID := "real01"
	ws, err := provider.Prepare(ctx, sessionID, tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}

	if !ws.IsIsolated {
		t.Fatal("ws.IsIsolated debió ser true")
	}
	if ws.BranchName != "harness/real01" {
		t.Errorf("BranchName esperado 'harness/real01', obtenido '%s'", ws.BranchName)
	}
	expectedWT := filepath.Join(tmpDir, ".harness", "worktrees", sessionID)
	if ws.WorktreeDir != expectedWT {
		t.Errorf("WorktreeDir esperado '%s', obtenido '%s'", expectedWT, ws.WorktreeDir)
	}
	if ws.TargetDir != expectedWT {
		t.Errorf("TargetDir esperado '%s', obtenido '%s'", expectedWT, ws.TargetDir)
	}

	// Comprobar que el directorio físico del worktree fue creado y contiene el archivo README.md
	isolatedReadme := filepath.Join(expectedWT, "README.md")
	data, err := os.ReadFile(isolatedReadme)
	if err != nil {
		t.Fatalf("el archivo en el worktree no pudo ser leído: %v", err)
	}
	if !strings.Contains(string(data), "Proyecto Principal") {
		t.Errorf("contenido inesperado en el worktree: %s", string(data))
	}

	// Escribir un archivo únicamente en el worktree (aislamiento)
	worktreeOnlyFile := filepath.Join(expectedWT, "isolated_agent_output.txt")
	if err := os.WriteFile(worktreeOnlyFile, []byte("cambio aislado"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verificar que el archivo NO existe en el directorio principal del usuario
	mainPathCheck := filepath.Join(tmpDir, "isolated_agent_output.txt")
	if _, err := os.Stat(mainPathCheck); !os.IsNotExist(err) {
		t.Error("el archivo generado en el worktree contaminó el directorio principal del usuario")
	}

	// 3. Probar re-preparación cuando el directorio del worktree ya existe
	wsResume, err := provider.Prepare(ctx, sessionID, tmpDir)
	if err != nil {
		t.Fatalf("Prepare con worktree existente falló: %v", err)
	}
	if !wsResume.IsIsolated || wsResume.WorktreeDir != expectedWT {
		t.Errorf("Prepare existente devolvió configuración errónea: %+v", wsResume)
	}

	// 4. Limpieza (Cleanup)
	err = provider.Cleanup(ctx, ws)
	if err != nil {
		t.Fatalf("Cleanup() falló: %v", err)
	}

	// Verificar que el worktree físico fue desmontado y eliminado
	if _, err := os.Stat(expectedWT); !os.IsNotExist(err) {
		t.Errorf("el directorio del worktree '%s' aún existe después de Cleanup()", expectedWT)
	}

	// Cleanup sobre workspace no aislado es idempotente y seguro
	nonIsolatedWS := &Workspace{
		WorkingDir: tmpDir,
		TargetDir:  tmpDir,
		IsIsolated: false,
	}
	if err := provider.Cleanup(ctx, nonIsolatedWS); err != nil {
		t.Errorf("Cleanup sobre ws no aislado no debió retornar error: %v", err)
	}
}

func TestGitProvider_RealNonGitDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "non-git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	provider := NewDefaultProvider()
	ctx := context.Background()

	if provider.IsGitAvailable(ctx, tmpDir) {
		t.Error("IsGitAvailable debió retornar false en directorio sin git")
	}

	ws, err := provider.Prepare(ctx, "sess_nongit", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}

	if ws.IsIsolated {
		t.Error("IsIsolated debe ser false en carpeta regular")
	}
	if ws.TargetDir != tmpDir {
		t.Errorf("TargetDir esperado '%s', obtenido '%s'", tmpDir, ws.TargetDir)
	}
	if ws.WorktreeDir != "" {
		t.Errorf("WorktreeDir esperado vacío, obtenido '%s'", ws.WorktreeDir)
	}
	if ws.BranchName != "" {
		t.Errorf("BranchName esperado vacío, obtenido '%s'", ws.BranchName)
	}

	// Cleanup no debe fallar
	if err := provider.Cleanup(ctx, ws); err != nil {
		t.Errorf("Cleanup() devolvió error inesperado: %v", err)
	}
}

func TestOSGitClient(t *testing.T) {
	client := &OSGitClient{}
	_, err := client.LookPath()
	if err != nil {
		t.Skip("git no disponible para test de OSGitClient")
	}

	out, err := client.Run(context.Background(), "", "version")
	if err != nil {
		t.Fatalf("OSGitClient.Run('version') falló: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("salida inesperada de git version: %s", out)
	}
}

func TestGitProvider_ExistingBranchRecovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wt-existing-branch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockGit := newMockGitClient()
	mockGit.runResponses["rev-parse --is-inside-work-tree"] = "true"
	mockGit.runResponses["rev-parse --git-dir"] = filepath.Join(tmpDir, ".git")

	expectedWT := filepath.Join(tmpDir, ".harness", "worktrees", "sess_existing")
	cmdAddB := "worktree add -b harness/sess_existing " + expectedWT
	cmdAddNoB := "worktree add " + expectedWT + " harness/sess_existing"

	mockGit.runErrors[cmdAddB] = errors.New("fatal: A branch named 'harness/sess_existing' already exists")
	mockGit.runResponses[cmdAddNoB] = "Preparing worktree"

	provider := NewProvider(mockGit)
	ws, err := provider.Prepare(context.Background(), "sess_existing", tmpDir)
	if err != nil {
		t.Fatalf("Prepare con rama existente debió recuperarse: %v", err)
	}
	if !ws.IsIsolated {
		t.Error("ws.IsIsolated debió ser true")
	}
}

func TestGitProvider_CleanupWorktreeErrors(t *testing.T) {
	mockGit := newMockGitClient()
	mockGit.runErrors["worktree remove --force /fake/wt"] = errors.New("error desmontando")
	mockGit.runErrors["branch -D harness/fake"] = errors.New("error borrando rama")

	provider := NewProvider(mockGit)
	err := provider.CleanupWorktree(context.Background(), "/fake/base", "/fake/wt", "harness/fake")
	if err == nil {
		t.Error("se esperaba error de cleanup")
	}
}

func TestGitProvider_EmptyBaseDir(t *testing.T) {
	mockGit := newMockGitClient()
	mockGit.lookPathErr = errors.New("no git")
	provider := NewProvider(mockGit)

	if provider.IsGitAvailable(context.Background(), "") {
		t.Error("debió ser false")
	}

	ws, err := provider.Prepare(context.Background(), "s1", "")
	if err != nil {
		t.Fatalf("Prepare con baseDir vacío falló: %v", err)
	}
	if ws.IsIsolated {
		t.Error("debió ser false")
	}
}

func TestResolveProjectRoot_Mock(t *testing.T) {
	ctx := context.Background()

	// 1. Subdirectorio dentro de repo Git -> debe resolver a la raíz
	mockGit := newMockGitClient()
	mockGit.runResponses["rev-parse --show-toplevel"] = "/fake/repo/root"

	resSub := ResolveProjectRoot(ctx, "/fake/repo/root/internal/cli", mockGit)
	if resSub != "/fake/repo/root" {
		t.Errorf("esperado '/fake/repo/root', obtenido '%s'", resSub)
	}

	// Método en GitProvider
	provider := NewProvider(mockGit)
	resProv := provider.ResolveProjectRoot(ctx, "/fake/repo/root/pkg/tool")
	if resProv != "/fake/repo/root" {
		t.Errorf("provider.ResolveProjectRoot esperado '/fake/repo/root', obtenido '%s'", resProv)
	}

	// 2. Raíz de un repo Git -> debe retornar la raíz
	resRoot := ResolveProjectRoot(ctx, "/fake/repo/root", mockGit)
	if resRoot != "/fake/repo/root" {
		t.Errorf("esperado '/fake/repo/root', obtenido '%s'", resRoot)
	}

	// 3. Directorio fuera de Git -> debe retornar el directorio provisto
	mockGitNon := newMockGitClient()
	mockGitNon.runErrors["rev-parse --show-toplevel"] = errors.New("fatal: not a git repository")

	resNonGit := ResolveProjectRoot(ctx, "/tmp/non-git-dir", mockGitNon)
	expectedClean := filepath.Clean("/tmp/non-git-dir")
	if resNonGit != expectedClean {
		t.Errorf("esperado '%s', obtenido '%s'", expectedClean, resNonGit)
	}

	// 4. Git no disponible en PATH -> retorna directorio original
	mockGitNoPath := newMockGitClient()
	mockGitNoPath.lookPathErr = errors.New("git no encontrado")
	resNoPath := ResolveProjectRoot(ctx, "/some/dir", mockGitNoPath)
	expectedNoPath := filepath.Clean("/some/dir")
	if resNoPath != expectedNoPath {
		t.Errorf("esperado '%s', obtenido '%s'", expectedNoPath, resNoPath)
	}
}

func TestResolveProjectRoot_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no está instalado en el PATH, saltando prueba real de ResolveProjectRoot")
	}

	ctx := context.Background()

	// Crear repositorio temporal real
	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")

	expectedRoot, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		expectedRoot = tmpDir
	}

	// 1. Directorio en la raíz de un repo git
	resRoot := ResolveProjectRoot(ctx, tmpDir)
	evalRoot, err := filepath.EvalSymlinks(resRoot)
	if err != nil {
		evalRoot = resRoot
	}
	if evalRoot != expectedRoot {
		t.Errorf("raíz real esperada '%s', obtenida '%s'", expectedRoot, evalRoot)
	}

	// 2. Subdirectorio dentro de un repo git (debe resolver a la raíz)
	subDir := filepath.Join(tmpDir, "internal", "deep", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	resSub := ResolveProjectRoot(ctx, subDir)
	evalSub, err := filepath.EvalSymlinks(resSub)
	if err != nil {
		evalSub = resSub
	}
	if evalSub != expectedRoot {
		t.Errorf("subdirectorio debió resolver a la raíz '%s', obtuvo '%s'", expectedRoot, evalSub)
	}

	// 3. Directorio fuera de git (debe retornar el directorio provisto)
	nonGitDir := t.TempDir()
	resNonGit := ResolveProjectRoot(ctx, nonGitDir)
	expectedNonGit, err := filepath.EvalSymlinks(nonGitDir)
	if err != nil {
		expectedNonGit = filepath.Clean(nonGitDir)
	}
	evalNonGit, err := filepath.EvalSymlinks(resNonGit)
	if err != nil {
		evalNonGit = resNonGit
	}
	if evalNonGit != expectedNonGit {
		t.Errorf("directorio fuera de git esperado '%s', obtenido '%s'", expectedNonGit, evalNonGit)
	}
}

func TestGitProvider_DiffWorktree_NonGit(t *testing.T) {
	tmpDir := t.TempDir()
	mockGit := newMockGitClient()
	mockGit.lookPathErr = errors.New("git not installed")

	provider := NewProvider(mockGit)
	_, err := provider.DiffWorktree(context.Background(), tmpDir, tmpDir+"/wt", "harness/s1", false)
	if err == nil {
		t.Fatal("DiffWorktree debió retornar error en directorio sin git")
	}
}

func TestGitProvider_DiffWorktree_Mock(t *testing.T) {
	tmpDir := t.TempDir()
	wtDir := filepath.Join(tmpDir, "wt")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatal(err)
	}

	mockGit := newMockGitClient()
	mockGit.runResponses["rev-parse --is-inside-work-tree"] = "true"
	mockGit.runResponses["merge-base HEAD harness/sess_mock"] = "abc1234"
	mockGit.runResponses["status --porcelain"] = "?? untracked.txt\n M modified.txt"
	mockGit.runResponses["add -N -- untracked.txt"] = ""
	mockGit.runResponses["reset -q -- untracked.txt"] = ""
	mockGit.runResponses["diff abc1234"] = "diff --git a/modified.txt b/modified.txt\n+change"
	mockGit.runResponses["diff --stat abc1234"] = "modified.txt | 1 +\n 1 file changed, 1 insertion(+)"

	provider := NewProvider(mockGit)
	ctx := context.Background()

	// 1. Diff completo
	out, err := provider.DiffWorktree(ctx, tmpDir, wtDir, "harness/sess_mock", false)
	if err != nil {
		t.Fatalf("DiffWorktree falló: %v", err)
	}
	if !strings.Contains(out, "+change") {
		t.Errorf("salida de diff inesperada: %s", out)
	}

	// 2. Diff con --stat
	outStat, err := provider.DiffWorktree(ctx, tmpDir, wtDir, "harness/sess_mock", true)
	if err != nil {
		t.Fatalf("DiffWorktree stat falló: %v", err)
	}
	if !strings.Contains(outStat, "modified.txt | 1 +") {
		t.Errorf("salida de diff stat inesperada: %s", outStat)
	}
}

func TestGitProvider_DiffWorktree_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test de DiffWorktree")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")

	// Commit base
	file1 := filepath.Join(tmpDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("linea 1\nlinea 2\n"), 0644)
	runGit(tmpDir, "add", "file1.txt")
	runGit(tmpDir, "commit", "-m", "init commit")

	provider := NewDefaultProvider()
	ctx := context.Background()

	// Preparar worktree
	ws, err := provider.Prepare(ctx, "diff_real", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	// En el worktree:
	// a) Modificación no confirmada en file1.txt
	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "file1.txt"), []byte("linea 1\nlinea 2 mod\n"), 0644)
	// b) Archivo untracked
	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "untracked.txt"), []byte("nuevo archivo\n"), 0644)
	// c) Commit intermedio
	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "committed.txt"), []byte("comiteado\n"), 0644)
	runGit(ws.WorktreeDir, "add", "committed.txt")
	runGit(ws.WorktreeDir, "commit", "-m", "commit en worktree")

	// 1. Diff completo
	diffOut, err := provider.DiffWorktree(ctx, tmpDir, ws.WorktreeDir, ws.BranchName, false)
	if err != nil {
		t.Fatalf("DiffWorktree falló: %v", err)
	}
	if !strings.Contains(diffOut, "file1.txt") {
		t.Errorf("diffOut debió contener file1.txt: %s", diffOut)
	}
	if !strings.Contains(diffOut, "untracked.txt") {
		t.Errorf("diffOut debió contener untracked.txt: %s", diffOut)
	}
	if !strings.Contains(diffOut, "committed.txt") {
		t.Errorf("diffOut debió contener committed.txt: %s", diffOut)
	}

	// 2. Diff con --stat
	statOut, err := provider.DiffWorktree(ctx, tmpDir, ws.WorktreeDir, ws.BranchName, true)
	if err != nil {
		t.Fatalf("DiffWorktree stat falló: %v", err)
	}
	if !strings.Contains(statOut, "file1.txt") || !strings.Contains(statOut, "untracked.txt") || !strings.Contains(statOut, "committed.txt") {
		t.Errorf("statOut no contiene todos los archivos: %s", statOut)
	}

	// Verificar que el estado untracked de untracked.txt se conservó
	status := runGit(ws.WorktreeDir, "status", "--porcelain")
	if !strings.Contains(status, "?? untracked.txt") {
		t.Errorf("untracked.txt debió quedar como untracked después del diff: %s", status)
	}
}

func TestGitProvider_MergeWorktree_NonGit(t *testing.T) {
	tmpDir := t.TempDir()
	mockGit := newMockGitClient()
	mockGit.lookPathErr = errors.New("git not installed")

	provider := NewProvider(mockGit)
	_, err := provider.MergeWorktree(context.Background(), "s1", tmpDir, tmpDir+"/wt", "harness/s1", false, false)
	if err == nil {
		t.Fatal("MergeWorktree debió retornar error en directorio sin git")
	}
}

func TestGitProvider_MergeWorktree_RealGit_Standard(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test de MergeWorktree")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")

	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "base commit")

	provider := NewDefaultProvider()
	ctx := context.Background()

	ws, err := provider.Prepare(ctx, "merge_std", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	// Crear archivo no comiteado en el worktree
	newFile := filepath.Join(ws.WorktreeDir, "from_agent.txt")
	_ = os.WriteFile(newFile, []byte("generado por agente\n"), 0644)

	// Ejecutar merge estándar
	res, err := provider.MergeWorktree(ctx, "merge_std", tmpDir, ws.WorktreeDir, ws.BranchName, false, false)
	if err != nil {
		t.Fatalf("MergeWorktree falló: %v", err)
	}

	if res.AlreadyUpToDate {
		t.Error("MergeWorktree no debió reportar AlreadyUpToDate con cambios pendientes")
	}
	if len(res.FilesIntegrated) == 0 || res.FilesIntegrated[0] != "from_agent.txt" {
		t.Errorf("FilesIntegrated esperado ['from_agent.txt'], obtenido: %v", res.FilesIntegrated)
	}

	// Comprobar que el archivo ahora existe físicamente en el repositorio base
	mainFileCheck := filepath.Join(tmpDir, "from_agent.txt")
	if _, err := os.Stat(mainFileCheck); os.IsNotExist(err) {
		t.Fatalf("el archivo integrado no se encuentra en el repositorio base: %s", mainFileCheck)
	}
}

func TestGitProvider_MergeWorktree_RealGit_Squash(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test de MergeWorktree squash")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")

	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "base commit")

	provider := NewDefaultProvider()
	ctx := context.Background()

	ws, err := provider.Prepare(ctx, "merge_squash", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	// Dos commits en el worktree
	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "f1.txt"), []byte("1\n"), 0644)
	runGit(ws.WorktreeDir, "add", "f1.txt")
	runGit(ws.WorktreeDir, "commit", "-m", "commit 1")

	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "f2.txt"), []byte("2\n"), 0644)
	runGit(ws.WorktreeDir, "add", "f2.txt")
	runGit(ws.WorktreeDir, "commit", "-m", "commit 2")

	// Merge con --squash (sin --no-commit)
	res, err := provider.MergeWorktree(ctx, "merge_squash", tmpDir, ws.WorktreeDir, ws.BranchName, true, false)
	if err != nil {
		t.Fatalf("MergeWorktree squash falló: %v", err)
	}

	if len(res.FilesIntegrated) != 2 {
		t.Errorf("FilesIntegrated esperado 2 archivos, obtenido: %v", res.FilesIntegrated)
	}

	// Comprobar que en el log principal solo se agregó 1 commit squash
	logOut := runGit(tmpDir, "log", "--oneline")
	lines := strings.Split(strings.TrimSpace(logOut), "\n")
	if len(lines) != 2 { // base commit + squash commit
		t.Errorf("esperado 2 commits en log (base + squash), obtenidos: %v", lines)
	}
}

func TestGitProvider_MergeWorktree_RealGit_NoCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test de MergeWorktree no-commit")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")

	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "base commit")

	provider := NewDefaultProvider()
	ctx := context.Background()

	ws, err := provider.Prepare(ctx, "merge_nocommit", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "staged.txt"), []byte("stage me\n"), 0644)
	runGit(ws.WorktreeDir, "add", "staged.txt")
	runGit(ws.WorktreeDir, "commit", "-m", "worktree commit")

	// Merge con --no-commit
	res, err := provider.MergeWorktree(ctx, "merge_nocommit", tmpDir, ws.WorktreeDir, ws.BranchName, false, true)
	if err != nil {
		t.Fatalf("MergeWorktree no-commit falló: %v", err)
	}

	if len(res.FilesIntegrated) != 1 {
		t.Errorf("FilesIntegrated esperado ['staged.txt'], obtenido: %v", res.FilesIntegrated)
	}

	// Comprobar que en el repositorio base los cambios están staged pero sin comitear
	statusOut := runGit(tmpDir, "status", "--porcelain")
	if !strings.Contains(statusOut, "A  staged.txt") {
		t.Errorf("el archivo debió quedar en el stage de baseDir: %s", statusOut)
	}
}

func TestGitProvider_MergeWorktree_AlreadyUpToDate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "base")

	provider := NewDefaultProvider()
	ctx := context.Background()

	ws, err := provider.Prepare(ctx, "merge_empty", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	// Sin cambios en el worktree
	res, err := provider.MergeWorktree(ctx, "merge_empty", tmpDir, ws.WorktreeDir, ws.BranchName, false, false)
	if err != nil {
		t.Fatalf("MergeWorktree falló: %v", err)
	}
	if !res.AlreadyUpToDate {
		t.Error("MergeWorktree debió reportar AlreadyUpToDate = true")
	}
}

func TestGitProvider_WorktreeGuardrails_PushAndBranchProtection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test de guardrails")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	// 1. Repo principal
	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "init")
	runGit(tmpDir, "branch", "develop")

	// 2. Repo remoto simulado (bare)
	remoteDir := t.TempDir()
	cmdInitBare := exec.Command("git", "init", "--bare")
	cmdInitBare.Dir = remoteDir
	_ = cmdInitBare.Run()
	runGit(tmpDir, "remote", "add", "origin", remoteDir)
	runGit(tmpDir, "push", "origin", "HEAD")

	provider := NewDefaultProvider()
	ctx := context.Background()

	// 3. Preparar worktree de sesión
	sessionID := "guard_sess"
	ws, err := provider.Prepare(ctx, sessionID, tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	// 4. Test commit legítimo dentro del worktree (debe funcionar)
	wtFile := filepath.Join(ws.WorktreeDir, "agent_work.txt")
	_ = os.WriteFile(wtFile, []byte("agent content\n"), 0644)
	runGit(ws.WorktreeDir, "add", "agent_work.txt")
	runGit(ws.WorktreeDir, "commit", "-m", "agent commit")

	// 5. Test git push desde el worktree (debe ser bloqueado por pre-push hook)
	cmdPush := exec.Command("git", "push", "origin", ws.BranchName)
	cmdPush.Dir = ws.WorktreeDir
	pushOut, pushErr := cmdPush.CombinedOutput()
	if pushErr == nil {
		t.Fatalf("git push debió ser bloqueado por el hook pre-push, pero tuvo éxito: %s", string(pushOut))
	}
	if !strings.Contains(string(pushOut), "estrictamente prohibido") {
		t.Errorf("salida esperada de pre-push no encontrada: %s", string(pushOut))
	}

	// 6. Test modificación de otra rama (develop) desde el worktree (debe ser bloqueado por reference-transaction)
	cmdBranch := exec.Command("git", "branch", "-f", "develop", "HEAD")
	cmdBranch.Dir = ws.WorktreeDir
	branchOut, branchErr := cmdBranch.CombinedOutput()
	if branchErr == nil {
		t.Fatalf("modificación de rama develop debió ser bloqueada por reference-transaction, pero tuvo éxito: %s", string(branchOut))
	}
	if !strings.Contains(string(branchOut), "Modificación prohibida") && !strings.Contains(string(branchOut), "update aborted") {
		t.Errorf("salida esperada de reference-transaction no encontrada: %s", string(branchOut))
	}

	// 7. Verificar que el repositorio base principal del humano NO tiene hooks activados y puede operar normalmente
	_ = os.WriteFile(filepath.Join(tmpDir, "human.txt"), []byte("human\n"), 0644)
	runGit(tmpDir, "add", "human.txt")
	runGit(tmpDir, "commit", "-m", "human commit")
	runGit(tmpDir, "branch", "-f", "develop", "HEAD")
}

func TestGitProvider_MergeWorktree_DirtyBaseDir_Fails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "base")

	provider := NewDefaultProvider()
	ctx := context.Background()

	ws, err := provider.Prepare(ctx, "merge_dirty", tmpDir)
	if err != nil {
		t.Fatalf("Prepare falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, ws) }()

	// El agente genera un archivo en el worktree
	_ = os.WriteFile(filepath.Join(ws.WorktreeDir, "agent.txt"), []byte("agent\n"), 0644)

	// El usuario ensucia su propio repo base sin comitear
	_ = os.WriteFile(filepath.Join(tmpDir, "uncommitted_user_file.txt"), []byte("modificado\n"), 0644)

	// Intentar mergear debe fallar con error amigable
	_, mergeErr := provider.MergeWorktree(ctx, "merge_dirty", tmpDir, ws.WorktreeDir, ws.BranchName, false, false)
	if mergeErr == nil {
		t.Fatal("MergeWorktree debió fallar porque el repositorio base tiene cambios sin comitear")
	}
	if !strings.Contains(mergeErr.Error(), "sin comitear") {
		t.Errorf("error inesperado: %v", mergeErr)
	}
}

func TestGitProvider_EnsureGitExcludeHarness(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test")
	}

	tmpDir := t.TempDir()
	cmdInit := exec.Command("git", "init")
	cmdInit.Dir = tmpDir
	_ = cmdInit.Run()

	provider := NewProvider()
	ctx := context.Background()

	err := provider.ensureGitExcludeHarness(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ensureGitExcludeHarness falló: %v", err)
	}

	excludePath := filepath.Join(tmpDir, ".git", "info", "exclude")
	content, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("no se pudo leer exclude: %v", err)
	}

	if !strings.Contains(string(content), ".harness") {
		t.Errorf("el archivo exclude no contiene .harness: %s", string(content))
	}
}

func TestGitProvider_Prune(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible para test de prune")
	}

	tmpDir := t.TempDir()
	runGit := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("error ejecutando git %v en %s: %v (%s)", args, dir, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit(tmpDir, "init")
	runGit(tmpDir, "config", "user.name", "Tester")
	runGit(tmpDir, "config", "user.email", "tester@example.com")
	_ = os.WriteFile(filepath.Join(tmpDir, "base.txt"), []byte("base\n"), 0644)
	runGit(tmpDir, "add", "base.txt")
	runGit(tmpDir, "commit", "-m", "init")

	provider := NewDefaultProvider()
	ctx := context.Background()

	// 1. Crear sesión activa
	wsActive, err := provider.Prepare(ctx, "active1", tmpDir)
	if err != nil {
		t.Fatalf("Prepare active1 falló: %v", err)
	}
	defer func() { _ = provider.Cleanup(ctx, wsActive) }()

	// 2. Crear rama huérfana harness/orphan1 (sin worktree)
	runGit(tmpDir, "branch", "harness/orphan1")

	// 3. Crear sesión que luego es huérfana (no estará en activeIDs)
	wsOrphan2, err := provider.Prepare(ctx, "orphan2", tmpDir)
	if err != nil {
		t.Fatalf("Prepare orphan2 falló: %v", err)
	}
	_ = wsOrphan2

	// 4. Ejecutar Prune pasando solo active1 como sesión activa
	report, err := provider.Prune(ctx, tmpDir, []string{"active1"})
	if err != nil {
		t.Fatalf("Prune falló: %v", err)
	}

	if len(report.DeletedBranches) == 0 {
		t.Errorf("Prune debió reportar ramas huérfanas eliminadas, obtuvo: %v", report.DeletedBranches)
	}
	if len(report.PrunedWorktrees) == 0 {
		t.Errorf("Prune debió reportar worktrees podados, obtuvo: %v", report.PrunedWorktrees)
	}

	// 5. Verificar que la rama active1 sigue existiendo y las huérfanas no
	branchesOut := runGit(tmpDir, "branch", "--list", "harness/*")
	if !strings.Contains(branchesOut, "harness/active1") {
		t.Errorf("la rama harness/active1 debió preservarse: %s", branchesOut)
	}
	if strings.Contains(branchesOut, "harness/orphan1") {
		t.Errorf("la rama harness/orphan1 debió ser eliminada: %s", branchesOut)
	}
	if strings.Contains(branchesOut, "harness/orphan2") {
		t.Errorf("la rama harness/orphan2 debió ser eliminada: %s", branchesOut)
	}
}


