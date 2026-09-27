package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitClient abstrae la ejecución de comandos Git para facilitar tests unitarios e inyección.
type GitClient interface {
	LookPath() (string, error)
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// OSGitClient ejecuta comandos de git en el sistema operativo.
type OSGitClient struct{}

// LookPath verifica si el ejecutable git está disponible en el PATH.
func (g *OSGitClient) LookPath() (string, error) {
	return exec.LookPath("git")
}

// Run ejecuta git con los argumentos provistos en el directorio de trabajo especificado.
func (g *OSGitClient) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Workspace representa el entorno de trabajo preparado para una sesión agéntica.
type Workspace struct {
	WorkingDir  string `json:"working_dir"`
	TargetDir   string `json:"target_dir"`
	IsIsolated  bool   `json:"is_isolated"`
	WorktreeDir string `json:"worktree_dir,omitempty"`
	BranchName  string `json:"branch_name,omitempty"`
}

// MergeResult contiene los detalles del resultado de una operación merge.
type MergeResult struct {
	Message         string   `json:"message"`
	FilesIntegrated []string `json:"files_integrated"`
	AlreadyUpToDate bool     `json:"already_up_to_date"`
}

// PruneReport detalla los elementos huérfanos eliminados durante la reconciliación.
type PruneReport struct {
	PrunedWorktrees []string `json:"pruned_worktrees"`
	DeletedBranches []string `json:"deleted_branches"`
}

// Provider define la interfaz para aprovisionar y limpiar workspaces (worktree o directo).
type Provider interface {
	IsGitAvailable(ctx context.Context, dir string) bool
	ResolveProjectRoot(ctx context.Context, dir string) string
	Prepare(ctx context.Context, sessionID string, baseDir string) (*Workspace, error)
	Cleanup(ctx context.Context, ws *Workspace) error
	CleanupWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string) error
	DiffWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string, statOnly bool) (string, error)
	MergeWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string, squash bool, noCommit bool) (*MergeResult, error)
	Prune(ctx context.Context, baseDir string, activeSessionIDs []string) (*PruneReport, error)
}

// GitProvider implementa Provider con detección automática y worktrees de Git.
type GitProvider struct {
	git GitClient
}

// NewProvider crea un GitProvider con el cliente Git especificado (o OSGitClient por defecto).
func NewProvider(git ...GitClient) *GitProvider {
	var g GitClient = &OSGitClient{}
	if len(git) > 0 && git[0] != nil {
		g = git[0]
	}
	return &GitProvider{git: g}
}

// ResolveProjectRoot detecta la raíz absoluta de un proyecto Git si aplica, o retorna el directorio limpio y absoluto.
func ResolveProjectRoot(ctx context.Context, dir string, git ...GitClient) string {
	var g GitClient = &OSGitClient{}
	if len(git) > 0 && git[0] != nil {
		g = git[0]
	}
	return resolveProjectRootWithClient(ctx, dir, g)
}

// ResolveProjectRoot retorna la raíz del repositorio Git o el directorio original limpio y absoluto.
func (p *GitProvider) ResolveProjectRoot(ctx context.Context, dir string) string {
	return resolveProjectRootWithClient(ctx, dir, p.git)
}

func resolveProjectRootWithClient(ctx context.Context, dir string, g GitClient) string {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			dir = "."
		}
	}

	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}
	dir = filepath.Clean(dir)

	if g != nil {
		if _, err := g.LookPath(); err == nil {
			out, err := g.Run(ctx, dir, "rev-parse", "--show-toplevel")
			if err == nil && out != "" {
				topLevel := strings.TrimSpace(out)
				if absTL, err := filepath.Abs(topLevel); err == nil {
					topLevel = absTL
				}
				return filepath.Clean(topLevel)
			}
		}
	}

	return dir
}

// NewDefaultProvider retorna la implementación por defecto de Provider.
func NewDefaultProvider() Provider {
	return NewProvider()
}

// IsGitAvailable comprueba si git está en el PATH y si dir es parte de un repositorio Git.
func (p *GitProvider) IsGitAvailable(ctx context.Context, dir string) bool {
	if _, err := p.git.LookPath(); err != nil {
		return false
	}

	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return false
		}
	}

	out, err := p.git.Run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil || out != "true" {
		return false
	}

	return true
}

// Prepare configura el workspace para la sesión. Si git está disponible, crea un worktree aislado;
// de lo contrario, realiza un fallback transparente operando sobre baseDir.
func (p *GitProvider) Prepare(ctx context.Context, sessionID string, baseDir string) (*Workspace, error) {
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error al obtener directorio de trabajo: %w", err)
		}
	}

	// Fallback directo si Git no está disponible o no es un repositorio
	if !p.IsGitAvailable(ctx, baseDir) {
		return &Workspace{
			WorkingDir:  baseDir,
			TargetDir:   baseDir,
			IsIsolated:  false,
			WorktreeDir: "",
			BranchName:  "",
		}, nil
	}

	// Verificar acceso al repositorio Git
	if _, err := p.git.Run(ctx, baseDir, "rev-parse", "--git-dir"); err != nil {
		// Fallback defensivo si ocurre un fallo al resolver .git
		return &Workspace{
			WorkingDir:  baseDir,
			TargetDir:   baseDir,
			IsIsolated:  false,
			WorktreeDir: "",
			BranchName:  "",
		}, nil
	}

	worktreesBase := filepath.Join(baseDir, ".harness", "worktrees")
	worktreeDir := filepath.Join(worktreesBase, sessionID)
	branchName := "harness/" + sessionID

	// Si el worktree ya existe físicamente en el disco (p.ej. reanudación previa)
	if fi, statErr := os.Stat(worktreeDir); statErr == nil && fi.IsDir() {
		_ = p.ensureGitExcludeHarness(ctx, baseDir)
		if gErr := p.installWorktreeGuardrails(ctx, baseDir, worktreeDir, sessionID); gErr != nil {
			return nil, fmt.Errorf("error al configurar guardrails en worktree existente: %w", gErr)
		}
		return &Workspace{
			WorkingDir:  baseDir,
			TargetDir:   worktreeDir,
			IsIsolated:  true,
			WorktreeDir: worktreeDir,
			BranchName:  branchName,
		}, nil
	}

	// Asegurar directorio base de worktrees (.harness/worktrees)
	if err := os.MkdirAll(worktreesBase, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio base de worktrees '%s': %w", worktreesBase, err)
	}

	// Asegurar que .harness esté excluido en .git/info/exclude del repositorio base
	_ = p.ensureGitExcludeHarness(ctx, baseDir)

	// Crear el worktree con la rama harness/<sessionID>
	_, err := p.git.Run(ctx, baseDir, "worktree", "add", "-b", branchName, worktreeDir)
	if err != nil {
		// Si la rama ya existía previamente, intentar asociar sin -b
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "ya existe") {
			_, err = p.git.Run(ctx, baseDir, "worktree", "add", worktreeDir, branchName)
		}
		if err != nil {
			return nil, fmt.Errorf("error al crear worktree en '%s': %w", worktreeDir, err)
		}
	}

	// Instalar guardrails de Git en el worktree (pre-push y reference-transaction)
	if gErr := p.installWorktreeGuardrails(ctx, baseDir, worktreeDir, sessionID); gErr != nil {
		return nil, fmt.Errorf("error al instalar guardrails de seguridad en el worktree: %w", gErr)
	}

	return &Workspace{
		WorkingDir:  baseDir,
		TargetDir:   worktreeDir,
		IsIsolated:  true,
		WorktreeDir: worktreeDir,
		BranchName:  branchName,
	}, nil
}

// Cleanup desmonta el worktree y borra la rama temporal del workspace si fue aislado.
func (p *GitProvider) Cleanup(ctx context.Context, ws *Workspace) error {
	if ws == nil || !ws.IsIsolated {
		return nil
	}
	return p.CleanupWorktree(ctx, ws.WorkingDir, ws.WorktreeDir, ws.BranchName)
}

// CleanupWorktree elimina el worktree físico y la rama temporal en Git.
func (p *GitProvider) CleanupWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string) error {
	var firstErr error

	if worktreeDir != "" {
		_, err := p.git.Run(ctx, baseDir, "worktree", "remove", "--force", worktreeDir)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		// Asegurar remoción completa del directorio en el sistema de archivos
		_ = os.RemoveAll(worktreeDir)
	}

	if branchName != "" {
		_, err := p.git.Run(ctx, baseDir, "branch", "-D", branchName)
		if err != nil && firstErr == nil {
			// No sobreescribir si ya se reportó error de worktree
			// Nota: Si la rama no existe, git retorna error que ignoramos defensivamente si el worktree ya se quitó
		}
	}

	// Podar referencias muertas en el metadata de worktrees
	_, _ = p.git.Run(ctx, baseDir, "worktree", "prune")

	// Si no quedan más worktrees ni ramas de harness, restaurar configuración del repositorio
	p.cleanupWorktreeConfigIfNoHarness(ctx, baseDir)

	return firstErr
}

// Prune reconcilia los worktrees y ramas de Git con las sesiones activas, eliminando basura huérfana.
func (p *GitProvider) Prune(ctx context.Context, baseDir string, activeSessionIDs []string) (*PruneReport, error) {
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error al obtener directorio base: %w", err)
		}
	}

	report := &PruneReport{
		PrunedWorktrees: []string{},
		DeletedBranches: []string{},
	}
	if !p.IsGitAvailable(ctx, baseDir) {
		return report, nil
	}

	// 1. Podar entradas muertas de worktrees en Git
	_, _ = p.git.Run(ctx, baseDir, "worktree", "prune")

	activeMap := make(map[string]bool)
	for _, id := range activeSessionIDs {
		activeMap[id] = true
	}

	// 1. Consultar todos los worktrees registrados en Git via porcelain
	wtListOut, err := p.git.Run(ctx, baseDir, "worktree", "list", "--porcelain")
	if err == nil {
		lines := strings.Split(wtListOut, "\n")
		var currentWTPath string
		var currentBranch string

		processBlock := func() {
			if currentWTPath != "" && strings.HasPrefix(currentBranch, "refs/heads/harness/") {
				sessID := strings.TrimPrefix(currentBranch, "refs/heads/harness/")
				if !activeMap[sessID] {
					_, _ = p.git.Run(ctx, baseDir, "worktree", "remove", "--force", currentWTPath)
					_ = os.RemoveAll(currentWTPath)
					_, _ = p.git.Run(ctx, baseDir, "branch", "-D", "harness/"+sessID)
					report.PrunedWorktrees = append(report.PrunedWorktrees, sessID)
					report.DeletedBranches = append(report.DeletedBranches, "harness/"+sessID)
				}
			}
			currentWTPath = ""
			currentBranch = ""
		}

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "worktree ") {
				processBlock()
				currentWTPath = strings.TrimSpace(strings.TrimPrefix(trimmed, "worktree "))
			} else if strings.HasPrefix(trimmed, "branch ") {
				currentBranch = strings.TrimSpace(strings.TrimPrefix(trimmed, "branch "))
			} else if trimmed == "" {
				processBlock()
			}
		}
		processBlock()
	}

	// 2. Podar referencias muertas en Git
	_, _ = p.git.Run(ctx, baseDir, "worktree", "prune")

	// 3. Reconciliar cualquier rama harness/* que aún pudiera haber quedado colgada sin worktree
	branchesOut, err := p.git.Run(ctx, baseDir, "branch", "--list", "harness/*")
	if err == nil {
		lines := strings.Split(branchesOut, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			trimmed = strings.TrimPrefix(trimmed, "*")
			trimmed = strings.TrimPrefix(trimmed, "+")
			trimmed = strings.TrimSpace(trimmed)
			if trimmed == "" || !strings.HasPrefix(trimmed, "harness/") {
				continue
			}

			sessID := strings.TrimPrefix(trimmed, "harness/")
			if !activeMap[sessID] {
				_, _ = p.git.Run(ctx, baseDir, "branch", "-D", trimmed)
				alreadyReported := false
				for _, b := range report.DeletedBranches {
					if b == trimmed {
						alreadyReported = true
						break
					}
				}
				if !alreadyReported {
					report.DeletedBranches = append(report.DeletedBranches, trimmed)
				}
			}
		}
	}

	// 4. Limpiar carpetas huérfanas en .harness/worktrees si hubieran quedado carpetas residuales
	worktreesBase := filepath.Join(baseDir, ".harness", "worktrees")
	if entries, err := os.ReadDir(worktreesBase); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sessID := entry.Name()
			if !activeMap[sessID] {
				wtPath := filepath.Join(worktreesBase, sessID)
				_ = os.RemoveAll(wtPath)
			}
		}
		if remaining, err := os.ReadDir(worktreesBase); err == nil && len(remaining) == 0 {
			_ = os.Remove(worktreesBase)
		}
	}

	// Si no quedan más worktrees ni ramas de harness, restaurar configuración del repositorio
	p.cleanupWorktreeConfigIfNoHarness(ctx, baseDir)

	return report, nil
}

// DiffWorktree obtiene las diferencias de código generadas en el worktree frente al commit base de referencia.
func (p *GitProvider) DiffWorktree(ctx context.Context, baseDir string, worktreeDir string, branchName string, statOnly bool) (string, error) {
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error al obtener directorio base: %w", err)
		}
	}

	if !p.IsGitAvailable(ctx, baseDir) {
		return "", errors.New("git no está disponible o el directorio no es un repositorio git")
	}

	// 1. Determinar referencia o commit base (merge-base con HEAD o el commit HEAD)
	var baseRef string
	if branchName != "" {
		baseCommit, err := p.git.Run(ctx, baseDir, "merge-base", "HEAD", branchName)
		if err == nil && strings.TrimSpace(baseCommit) != "" {
			baseRef = strings.TrimSpace(baseCommit)
		}
	}
	if baseRef == "" {
		headCommit, err := p.git.Run(ctx, baseDir, "rev-parse", "HEAD")
		if err == nil && strings.TrimSpace(headCommit) != "" {
			baseRef = strings.TrimSpace(headCommit)
		} else {
			baseRef = "HEAD"
		}
	}

	// 2. Si el worktree existe físicamente en disco, inspeccionar directamente en su directorio
	if worktreeDir != "" {
		if fi, err := os.Stat(worktreeDir); err == nil && fi.IsDir() {
			sessionID := strings.TrimPrefix(branchName, "harness/")
			manifest, _ := LoadManifest(baseDir, sessionID)

			diffArgs := []string{"diff"}
			if statOnly {
				diffArgs = append(diffArgs, "--stat")
			}
			diffArgs = append(diffArgs, baseRef)

			if manifest != nil {
				var pathspecs []string
				for relPath, expectedHash := range manifest.ProjectedHash {
					filePath := filepath.Join(worktreeDir, relPath)
					if curBytes, err := os.ReadFile(filePath); err == nil {
						if HashBytes(curBytes) == expectedHash {
							pathspecs = append(pathspecs, ":!"+relPath)
						}
					}
				}
				if len(pathspecs) > 0 {
					diffArgs = append(diffArgs, "--")
					diffArgs = append(diffArgs, pathspecs...)
				}
			}

			trackedDiff, tErr := p.git.Run(ctx, worktreeDir, diffArgs...)
			if tErr != nil {
				return "", fmt.Errorf("error al obtener diff en worktree: %w", tErr)
			}

			// Detectar archivos untracked usando git status --porcelain sin alterar el índice de git
			statusOut, sErr := p.git.Run(ctx, worktreeDir, "status", "--porcelain")
			if sErr != nil {
				return "", fmt.Errorf("error al obtener status en worktree: %w", sErr)
			}
			var untrackedDiffs []string

			// Comprobar si AGENTS.md está trackeado en el repositorio base HEAD
			trackedAgents := false
			if _, chkErr := p.git.Run(ctx, baseDir, "cat-file", "-e", "HEAD:AGENTS.md"); chkErr == nil {
				trackedAgents = true
			}

			for _, line := range strings.Split(statusOut, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "?? ") {
					f := strings.TrimSpace(line[3:])
					f = strings.Trim(f, "\"")
					if f == "" {
						continue
					}
					if manifest != nil {
						isCreated := false
						for _, cf := range manifest.CreatedFiles {
							if cf == f || strings.HasPrefix(f, cf+"/") || strings.HasPrefix(f, cf+"\\") {
								isCreated = true
								break
							}
						}
						if isCreated {
							if expectedHash, hasHash := manifest.ProjectedHash[f]; hasHash {
								filePath := filepath.Join(worktreeDir, f)
								if curBytes, err := os.ReadFile(filePath); err == nil {
									if HashBytes(curBytes) == expectedHash {
										continue
									}
								}
							} else {
								continue
							}
						}
					} else {
						// Ignorar artefactos efímeros proyectados de gz-ia
						if isHarnessArtifact(f) {
							continue
						}
						if f == "AGENTS.md" && !trackedAgents {
							continue
						}
					}

					var uArgs []string
					if statOnly {
						uArgs = []string{"diff", "--no-index", "--stat", "--", "/dev/null", f}
					} else {
						uArgs = []string{"diff", "--no-index", "--", "/dev/null", f}
					}
					uDiff, _ := p.git.Run(ctx, worktreeDir, uArgs...)
					if strings.TrimSpace(uDiff) != "" {
						untrackedDiffs = append(untrackedDiffs, strings.TrimSpace(uDiff))
					}
				}
			}

			result := strings.TrimSpace(trackedDiff)
			if len(untrackedDiffs) > 0 {
				joinedUntracked := strings.Join(untrackedDiffs, "\n\n")
				if result != "" {
					result = result + "\n\n" + joinedUntracked
				} else {
					result = joinedUntracked
				}
			}
			return strings.TrimSpace(result), nil
		}
	}

	// 3. Si el worktree físico ya no existe pero la rama sí está presente en git
	if branchName != "" {
		diffArgs := []string{"diff"}
		if statOnly {
			diffArgs = append(diffArgs, "--stat")
		}
		diffArgs = append(diffArgs, baseRef+".."+branchName)
		out, err := p.git.Run(ctx, baseDir, diffArgs...)
		if err != nil {
			return "", fmt.Errorf("error al ejecutar git diff entre ramas: %w", err)
		}
		return strings.TrimSpace(out), nil
	}

	return "", errors.New("worktree o rama no disponible")
}

// MergeWorktree integra los cambios de la rama del worktree en la rama activa del repositorio base.
// Semántica técnica:
// 1. Si hay cambios pendientes en el worktree, crea un commit de seguridad en la rama harness/<id>.
// 2. Ejecuta un git merge (o git merge --squash con squash, y sin commit si se pasa noCommit) de la rama harness/<id> en la rama base activa.
// 3. Si la rama base avanzó y existen conflictos, Git se detiene sin sobreescribir tus archivos; informa los archivos en conflicto y mantiene el worktree intacto para resolución manual (gz-ia session path <id>) o abortar (git merge --abort).
func (p *GitProvider) MergeWorktree(ctx context.Context, sessionID string, baseDir string, worktreeDir string, branchName string, squash bool, noCommit bool) (*MergeResult, error) {
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("error al obtener directorio de trabajo: %w", err)
		}
	}

	if !p.IsGitAvailable(ctx, baseDir) {
		return nil, errors.New("git no está disponible o el directorio no es un repositorio git")
	}

	// 0. Verificar que el repositorio base esté limpio para no arriesgar ni sobreescribir los archivos del usuario
	baseStatus, _ := p.git.Run(ctx, baseDir, "status", "--porcelain")
	if isWorkingTreeDirty(baseStatus) {
		return nil, fmt.Errorf("el repositorio base ('%s') tiene modificaciones o archivos sin comitear.\nPor favor realiza commit o stash antes de integrar los cambios de la sesión para evitar pérdida accidental de datos", baseDir)
	}

	if branchName == "" {
		branchName = "harness/" + sessionID
	}

	// 1. Si en el worktree hay archivos sin comitear (uncommitted/untracked), prepararlos y comitearlos en la rama del worktree
	if worktreeDir != "" {
		if fi, err := os.Stat(worktreeDir); err == nil && fi.IsDir() {
			manifest, _ := LoadManifest(baseDir, sessionID)
			if manifest != nil {
				// A. Restaurar archivos originales (incluso si fueron editados por el agente, para proteger la rama del usuario)
				for relPath, origContent := range manifest.OriginalFiles {
					filePath := filepath.Join(worktreeDir, relPath)
					if curBytes, err := os.ReadFile(filePath); err == nil {
						curHash := HashBytes(curBytes)
						expectedHash, ok := manifest.ProjectedHash[relPath]
						origHash := HashBytes([]byte(origContent))
						if ok && curHash == expectedHash {
							_ = os.WriteFile(filePath, []byte(origContent), 0644)
						} else if curHash != origHash {
							// El archivo proyectado fue editado por el agente pero no coincide con el original.
							// Para no filtrar credenciales ni wrappers del sistema en la rama del usuario,
							// restauramos el contenido original y advertimos.
							_ = os.WriteFile(filePath, []byte(origContent), 0644)
							fmt.Fprintf(os.Stderr, "ADVERTENCIA: el archivo proyectado '%s' fue modificado por el agente. Se restauró su contenido original para evitar fuga de configuraciones internas o credenciales en la rama base.\n", relPath)
						}
					}
				}

				// B. Eliminar archivos creados por gz-ia que no existían originalmente
				for _, relPath := range manifest.CreatedFiles {
					filePath := filepath.Join(worktreeDir, relPath)
					if curBytes, err := os.ReadFile(filePath); err == nil {
						expectedHash, ok := manifest.ProjectedHash[relPath]
						if ok && HashBytes(curBytes) != expectedHash {
							fmt.Fprintf(os.Stderr, "ADVERTENCIA: el archivo proyectado '%s' creado por gz-ia fue modificado por el agente. Se elimina para proteger la rama base.\n", relPath)
						}
						_ = os.Remove(filePath)
						_, _ = p.git.Run(ctx, worktreeDir, "rm", "-f", "--cached", "--ignore-unmatch", relPath)
					} else {
						// Symlink o directorio
						if _, lerr := os.Lstat(filePath); lerr == nil {
							_ = os.RemoveAll(filePath)
							_, _ = p.git.Run(ctx, worktreeDir, "rm", "-rf", "--cached", "--ignore-unmatch", relPath)
						}
					}
				}
				_ = removeEmptyDirs(filepath.Join(worktreeDir, ".agents"))
			} else {
				// Si no hay manifiesto, procedemos de forma segura sin borrar archivos
				// que podrían pertenecer al repositorio del usuario (sin fallback destructivo).
			}

			statusOut, _ := p.git.Run(ctx, worktreeDir, "status", "--porcelain")
			if strings.TrimSpace(statusOut) != "" {
				if _, err := p.git.Run(ctx, worktreeDir, "add", "-A"); err != nil {
					return nil, fmt.Errorf("error al preparar cambios en worktree: %w", err)
				}
				commitMsg := fmt.Sprintf("chore(harness): session %s changes", sessionID)
				if _, err := p.git.Run(ctx, worktreeDir, "commit", "--no-verify", "-m", commitMsg); err != nil {
					// Fallback si falta git config de usuario
					if strings.Contains(err.Error(), "user.name") || strings.Contains(err.Error(), "tell me who you are") {
						_, err = p.git.Run(ctx, worktreeDir, "-c", "user.name=gz-ia", "-c", "user.email=gz-ia@localhost", "commit", "--no-verify", "-m", commitMsg)
					}
					if err != nil {
						return nil, fmt.Errorf("error al comitear cambios en worktree: %w", err)
					}
				}
			}
		}
	}

	// 2. Extraer lista de archivos que diferencian a branchName respecto a HEAD en baseDir
	var integratedFiles []string
	diffNamesOut, _ := p.git.Run(ctx, baseDir, "diff", "--name-only", "HEAD..."+branchName)
	for _, f := range strings.Split(diffNamesOut, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			integratedFiles = append(integratedFiles, f)
		}
	}

	// 3. Ejecutar merge en baseDir según flags
	var mergeOut string
	var mergeErr error

	if squash {
		mergeOut, mergeErr = p.git.Run(ctx, baseDir, "merge", "--squash", branchName)
		if mergeErr != nil {
			if strings.Contains(strings.ToLower(mergeOut), "conflict") {
				return nil, fmt.Errorf("conflicto al fusionar cambios con --squash: Git detuvo la operación sin sobreescribir archivos.\n%s\nPuedes resolver los conflictos manualmente o abortar con 'git reset --merge'. El worktree permanece intacto en %s", strings.TrimSpace(mergeOut), worktreeDir)
			}
			return nil, fmt.Errorf("error al ejecutar git merge --squash: %w (%s)", mergeErr, mergeOut)
		}
		if !noCommit {
			st, _ := p.git.Run(ctx, baseDir, "status", "--porcelain")
			if strings.TrimSpace(st) != "" {
				commitMsg := fmt.Sprintf("chore(harness): merge session %s changes (squash)", sessionID)
				cOut, cErr := p.git.Run(ctx, baseDir, "commit", "--no-verify", "-m", commitMsg)
				if cErr != nil && (strings.Contains(cErr.Error(), "user.name") || strings.Contains(cErr.Error(), "tell me who you are")) {
					cOut, cErr = p.git.Run(ctx, baseDir, "-c", "user.name=gz-ia", "-c", "user.email=gz-ia@localhost", "commit", "--no-verify", "-m", commitMsg)
				}
				if cErr != nil {
					return nil, fmt.Errorf("error al comitear squash merge: %w (%s)", cErr, cOut)
				}
				mergeOut = mergeOut + "\n" + cOut
			}
		}
	} else if noCommit {
		mergeOut, mergeErr = p.git.Run(ctx, baseDir, "merge", "--no-ff", "--no-commit", branchName)
		if mergeErr != nil {
			if strings.Contains(strings.ToLower(mergeOut), "conflict") {
				return nil, fmt.Errorf("conflicto al fusionar cambios con --no-commit: Git detuvo la operación sin sobreescribir archivos.\n%s\nPuedes resolver los conflictos manualmente o abortar con 'git merge --abort'. El worktree permanece intacto en %s", strings.TrimSpace(mergeOut), worktreeDir)
			}
			return nil, fmt.Errorf("error al ejecutar git merge --no-commit: %w (%s)", mergeErr, mergeOut)
		}
	} else {
		commitMsg := fmt.Sprintf("chore(harness): merge session %s changes", sessionID)
		mergeOut, mergeErr = p.git.Run(ctx, baseDir, "merge", "--no-verify", branchName, "-m", commitMsg)
		if mergeErr != nil && strings.Contains(mergeErr.Error(), "unknown option") {
			mergeOut, mergeErr = p.git.Run(ctx, baseDir, "merge", branchName, "-m", commitMsg)
		}
		if mergeErr != nil {
			if strings.Contains(strings.ToLower(mergeOut), "conflict") {
				return nil, fmt.Errorf("conflicto al fusionar cambios: Git detuvo el merge sin sobreescribir archivos.\n%s\nPuedes resolver los conflictos manualmente o abortar con 'git merge --abort'. El worktree permanece intacto (inspecciona con 'gz-ia session path %s')", strings.TrimSpace(mergeOut), sessionID)
			}
			return nil, fmt.Errorf("error al ejecutar git merge: %w (%s)", mergeErr, mergeOut)
		}
	}

	lowerOut := strings.ToLower(mergeOut)
	alreadyUpToDate := len(integratedFiles) == 0 ||
		strings.Contains(lowerOut, "already up to date") ||
		strings.Contains(lowerOut, "ya está actualizado")

	return &MergeResult{
		Message:         strings.TrimSpace(mergeOut),
		FilesIntegrated: integratedFiles,
		AlreadyUpToDate: alreadyUpToDate,
	}, nil
}

// isWorkingTreeDirty evalúa si git status --porcelain tiene modificaciones en archivos trackeados (staged o unstaged).
// Ignora líneas de archivos sin seguimiento (??), así como rutas dentro de .harness.
func isWorkingTreeDirty(statusOut string) bool {
	lines := strings.Split(statusOut, "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		// Ignorar archivos untracked (??)
		if strings.HasPrefix(line, "??") {
			continue
		}
		path := strings.TrimSpace(line[3:])
		path = strings.Trim(path, "\"")
		if path == ".harness" || strings.HasPrefix(path, ".harness/") {
			continue
		}
		return true
	}
	return false
}

// ensureGitExcludeHarness asegura que .harness esté listado en .git/info/exclude
// usando 'git rev-parse --git-path info/exclude' para resolver la ruta real incluso
// en repositorios normales, submódulos o worktrees donde .git es un puntero o archivo.
func (p *GitProvider) ensureGitExcludeHarness(ctx context.Context, baseDir string) error {
	excludePathOut, err := p.git.Run(ctx, baseDir, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return nil
	}
	excludePath := strings.TrimSpace(excludePathOut)
	if excludePath == "" {
		return nil
	}
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(baseDir, excludePath)
	}

	infoDir := filepath.Dir(excludePath)
	_ = os.MkdirAll(infoDir, 0755)

	content, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return nil
	}

	strContent := string(content)
	for _, line := range strings.Split(strContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".harness" || trimmed == ".harness/" {
			return nil
		}
	}

	newContent := strContent
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += "\n# gz-ia session worktrees and metadata\n.harness\n.harness/\n"
	_ = os.WriteFile(excludePath, []byte(newContent), 0644)
	return nil
}

// installWorktreeGuardrails configura hooks locales y per-worktree configs en el worktree
// para evitar que el agente (incluso en autonomous) pueda modificar ramas fuera de harness/<sessionID> o hacer git push,
// reenviando además los hooks de pre-commit y commit-msg del proyecto si existen.
func (p *GitProvider) installWorktreeGuardrails(ctx context.Context, baseDir string, worktreeDir string, sessionID string) error {
	wtCfgOut, _ := p.git.Run(ctx, baseDir, "config", "--get", "extensions.worktreeConfig")
	if strings.TrimSpace(wtCfgOut) != "true" {
		if _, err := p.git.Run(ctx, baseDir, "config", "extensions.worktreeConfig", "true"); err != nil {
			return fmt.Errorf("error al activar extensions.worktreeConfig en baseDir: %w", err)
		}
		markerPath := filepath.Join(baseDir, ".harness", ".worktree_config_set")
		_ = os.MkdirAll(filepath.Dir(markerPath), 0755)
		_ = os.WriteFile(markerPath, []byte("true"), 0644)
	}

	gitDirOut, err := p.git.Run(ctx, worktreeDir, "rev-parse", "--git-dir")
	if err != nil {
		return fmt.Errorf("error al obtener git-dir en worktree: %w", err)
	}
	gitDir := strings.TrimSpace(gitDirOut)
	if gitDir == "" {
		return errors.New("git-dir de worktree vacío")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreeDir, gitDir)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de hooks en worktree: %w", err)
	}

	prePushScript := `#!/bin/sh
echo "[gz-ia GUARD] 'git push' está estrictamente prohibido desde un worktree de sesión de gz-ia." >&2
echo "[gz-ia GUARD] La integración y publicación al remoto deben realizarse por el operador humano en el repositorio principal." >&2
exit 1
`
	refTxScript := fmt.Sprintf(`#!/bin/sh
state="$1"
if [ "$state" = "prepared" ]; then
    while read -r old new refname; do
        case "$refname" in
            refs/heads/harness/%s|HEAD|ORIG_HEAD|FETCH_HEAD|MERGE_HEAD|AUTO_MERGE|CHERRY_PICK_HEAD|REVERT_HEAD)
                ;;
            *)
                echo "[gz-ia GUARD] Modificación prohibida: la referencia '$refname' no puede ser modificada desde este worktree." >&2
                echo "[gz-ia GUARD] El agente solo tiene permitido operar dentro de 'refs/heads/harness/%s'." >&2
                exit 1
                ;;
        esac
    done
fi
exit 0
`, sessionID, sessionID)

	prePushPath := filepath.Join(hooksDir, "pre-push")
	_ = os.WriteFile(prePushPath, []byte(prePushScript), 0755)
	_ = os.Chmod(prePushPath, 0755)

	refTxPath := filepath.Join(hooksDir, "reference-transaction")
	_ = os.WriteFile(refTxPath, []byte(refTxScript), 0755)
	_ = os.Chmod(refTxPath, 0755)

	// Resolver core.hooksPath del repo original para reenviar hooks respetando su entorno y wrappers (husky)
	origHooksPathOut, _ := p.git.Run(ctx, baseDir, "config", "--get", "core.hooksPath")
	origHooksPath := strings.TrimSpace(origHooksPathOut)
	if origHooksPath == "" {
		origHooksPath = filepath.Join(baseDir, ".git", "hooks")
	} else if !filepath.IsAbs(origHooksPath) {
		origHooksPath = filepath.Join(baseDir, origHooksPath)
	}

	forwardHook := func(hookName string) {
		targetHook := filepath.Join(hooksDir, hookName)
		origHook := filepath.Join(origHooksPath, hookName)
		hookScript := fmt.Sprintf(`#!/bin/sh
# Reenviador automático de gz-ia hacia hooks del proyecto principal
orig_hook="%s"
if [ -x "$orig_hook" ]; then
    "$orig_hook" "$@"
    exit $?
elif [ -f "$orig_hook" ]; then
    sh "$orig_hook" "$@"
    exit $?
fi
exit 0
`, origHook)
		_ = os.WriteFile(targetHook, []byte(hookScript), 0755)
		_ = os.Chmod(targetHook, 0755)
	}

	forwardHook("commit-msg")
	forwardHook("pre-commit")

	if _, err := p.git.Run(ctx, worktreeDir, "config", "--worktree", "core.hooksPath", hooksDir); err != nil {
		return fmt.Errorf("error al configurar core.hooksPath en worktree: %w", err)
	}

	return nil
}

func (p *GitProvider) cleanupWorktreeConfigIfNoHarness(ctx context.Context, baseDir string) {
	markerPath := filepath.Join(baseDir, ".harness", ".worktree_config_set")
	if _, statErr := os.Stat(markerPath); statErr != nil {
		return
	}
	branchesOut, err := p.git.Run(ctx, baseDir, "branch", "--list", "harness/*")
	if err == nil && strings.TrimSpace(branchesOut) == "" {
		wtOut, wtErr := p.git.Run(ctx, baseDir, "worktree", "list", "--porcelain")
		if wtErr == nil && !strings.Contains(wtOut, "/.harness/worktrees/") && !strings.Contains(wtOut, "\\.harness\\worktrees\\") {
			_, _ = p.git.Run(ctx, baseDir, "config", "--unset", "extensions.worktreeConfig")
			_ = os.Remove(markerPath)
		}
	}
}

func removeEmptyDirs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			_ = removeEmptyDirs(filepath.Join(dir, entry.Name()))
		}
	}
	remaining, err := os.ReadDir(dir)
	if err == nil && len(remaining) == 0 {
		_ = os.Remove(dir)
	}
	return nil
}

func isHarnessArtifact(path string) bool {
	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	if clean == ".agents" || strings.HasPrefix(clean, ".agents/") || strings.HasPrefix(clean, ".agents\\") {
		return true
	}
	if base == ".mcp.json" {
		return true
	}
	if strings.HasSuffix(base, "-AGENTS.md") {
		return true
	}
	return false
}
