package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gz-ia/internal/features/workspace"
	"gz-ia/packages/orchy"
	"gz-ia/packages/orchy/tools"
)

// Service define el contrato de operaciones para el ecosistema de tooling y perfiles/presets modulares.
type Service interface {
	ListToolkits(ctx context.Context) ([]Toolkit, error)
	GetToolkit(ctx context.Context, id string) (*Toolkit, error)
	CreateToolkit(ctx context.Context, req CreateToolkitRequest) (*Toolkit, error)
	ListSkills(ctx context.Context) ([]SkillInfo, error)
	ListPresets(ctx context.Context) ([]Preset, error)
	GetPreset(ctx context.Context, name string) (*Preset, error)
	ResolveToolkits(ctx context.Context, names []string) ([]string, error)
	ComposeToolkits(ctx context.Context, toolkitIDs []string) (*ComposedTooling, error)
	RegisterToolsInKernel(ctx context.Context, kernel *orchy.Kernel, tooling *ComposedTooling, workDir string) error
	ProjectIntoWorktree(ctx context.Context, targetDir string, composed *ComposedTooling, sessionID string, baseDir ...string) error
	ToolingDir() string
	ProjectDir() string

	// Métodos de compatibilidad histórica
	ListProfiles(ctx context.Context) ([]ProfileConfig, error)
	GetProfile(ctx context.Context, name string) (*ProfileConfig, error)
}

type toolingService struct {
	loader *Loader
}

// NewService crea un nuevo servicio de tooling con el loader configurado.
// Admite parámetros opcionales: toolingDir (global) y projectDir.
func NewService(dirs ...string) Service {
	toolingDir := ""
	projectDir := ""
	if len(dirs) > 0 && dirs[0] != "" {
		toolingDir = dirs[0]
	}
	if len(dirs) > 1 && dirs[1] != "" {
		projectDir = dirs[1]
	}
	return &toolingService{
		loader: NewLoader(toolingDir, projectDir),
	}
}

func (s *toolingService) ToolingDir() string {
	return s.loader.ToolingDir()
}

func (s *toolingService) ProjectDir() string {
	return s.loader.ProjectDir()
}

// ListToolkits descubre todos los toolkits disponibles en el proyecto y entorno global.
func (s *toolingService) ListToolkits(ctx context.Context) ([]Toolkit, error) {
	return s.loader.ListToolkits()
}

// GetToolkit busca un toolkit por su ID.
func (s *toolingService) GetToolkit(ctx context.Context, id string) (*Toolkit, error) {
	return s.loader.LoadToolkit(id)
}

// CreateToolkit genera el scaffolding completo para un nuevo toolkit.
func (s *toolingService) CreateToolkit(ctx context.Context, req CreateToolkitRequest) (*Toolkit, error) {
	return s.loader.CreateToolkitScaffold(req)
}

// ListSkills lista todas las habilidades descubiertas en toolkits y catálogos.
func (s *toolingService) ListSkills(ctx context.Context) ([]SkillInfo, error) {
	return s.loader.ListSkills()
}

// ListPresets retorna la lista de presets declarados en config.json.
func (s *toolingService) ListPresets(ctx context.Context) ([]Preset, error) {
	cfg, err := s.loader.LoadConfig()
	if err != nil {
		return nil, err
	}
	if len(cfg.Presets) > 0 {
		return cfg.Presets, nil
	}
	return cfg.Perfiles, nil
}

// GetPreset busca un preset por nombre exacto o normalizado (case-insensitive).
func (s *toolingService) GetPreset(ctx context.Context, name string) (*Preset, error) {
	clean := strings.TrimSpace(strings.ToLower(name))
	presets, err := s.ListPresets(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range presets {
		if strings.ToLower(p.Name) == clean {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("preset de tooling '%s' no encontrado", name)
}

// ResolveToolkits toma una lista de identificadores (que pueden ser presets o toolkits) y retorna
// la lista unificada y deduplicada de toolkit IDs listos para composición.
func (s *toolingService) ResolveToolkits(ctx context.Context, names []string) ([]string, error) {
	var resolved []string
	seen := make(map[string]bool)

	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}

		// 1. Probar si coincide con un Preset
		preset, err := s.GetPreset(ctx, name)
		if err == nil && preset != nil {
			for _, tkID := range preset.Toolkits {
				cleanTK := strings.TrimSpace(tkID)
				if cleanTK != "" && !seen[cleanTK] {
					seen[cleanTK] = true
					resolved = append(resolved, cleanTK)
				}
			}
			continue
		}

		// 2. Probar si coincide directamente con un Toolkit
		tk, err := s.GetToolkit(ctx, name)
		if err == nil && tk != nil {
			if !seen[tk.ID] {
				seen[tk.ID] = true
				resolved = append(resolved, tk.ID)
			}
			continue
		}

		return nil, fmt.Errorf("no se encontró ningún preset ni toolkit con el identificador '%s'", name)
	}

	return resolved, nil
}

// ListProfiles mantiene compatibilidad histórica delegando en ListPresets.
func (s *toolingService) ListProfiles(ctx context.Context) ([]ProfileConfig, error) {
	return s.ListPresets(ctx)
}

// GetProfile mantiene compatibilidad histórica delegando en GetPreset.
func (s *toolingService) GetProfile(ctx context.Context, name string) (*ProfileConfig, error) {
	return s.GetPreset(ctx, name)
}

// ComposeToolkits unifica las directivas, reglas, skills y herramientas de los toolkits solicitados.
func (s *toolingService) ComposeToolkits(ctx context.Context, toolkitIDs []string) (*ComposedTooling, error) {
	composed := &ComposedTooling{
		ActiveToolkits: []string{},
		AgentsFiles:    make(map[string]string),
		RulesFiles:     make(map[string]string),
		SkillPaths:     make(map[string]string),
		Tools:          []DeclaredTool{},
		MCPServers:     make(map[string]any),
		Env:            make(map[string]string),
	}

	seenToolkits := make(map[string]bool)
	seenTools := make(map[string]bool)

	for _, rawID := range toolkitIDs {
		id := strings.TrimSpace(rawID)
		if id == "" || seenToolkits[id] {
			continue
		}
		seenToolkits[id] = true

		tk, err := s.loader.LoadToolkit(id)
		if err != nil {
			return nil, err
		}

		composed.ActiveToolkits = append(composed.ActiveToolkits, tk.ID)

		// 1. Directivas AGENTS.md
		if tk.AgentsPath != "" {
			prefix := strings.ToUpper(strings.ReplaceAll(tk.ID, "-", "_"))
			targetName := fmt.Sprintf("%s-AGENTS.md", prefix)
			if existing, conflict := composed.AgentsFiles[targetName]; conflict && existing != tk.AgentsPath {
				return nil, fmt.Errorf("colisión de directivas agénticas: el archivo '%s' ya fue definido por otro toolkit (%s vs %s)", targetName, existing, tk.AgentsPath)
			}
			composed.AgentsFiles[targetName] = tk.AgentsPath
		}

		// 2. Reglas Markdown
		for ruleName, rulePath := range tk.RulesPaths {
			if existing, conflict := composed.RulesFiles[ruleName]; conflict && existing != rulePath {
				return nil, fmt.Errorf("colisión de reglas en tooling: la regla '%s' está definida en múltiples toolkits (%s vs %s)", ruleName, existing, rulePath)
			}
			composed.RulesFiles[ruleName] = rulePath
		}

		// 3. Skills
		for skillName, skillPath := range tk.SkillPaths {
			if existing, conflict := composed.SkillPaths[skillName]; conflict && existing != skillPath {
				return nil, fmt.Errorf("colisión de skills en tooling: el skill '%s' está definido en múltiples toolkits (%s vs %s)", skillName, existing, skillPath)
			}
			composed.SkillPaths[skillName] = skillPath
		}

		// 4. Herramientas MCP
		for _, tool := range tk.Tools {
			if seenTools[tool.Name] {
				return nil, fmt.Errorf("colisión de herramientas MCP en tooling: la herramienta '%s' está definida en múltiples toolkits", tool.Name)
			}
			seenTools[tool.Name] = true
			composed.Tools = append(composed.Tools, tool)
		}

		// 5. Servidores MCP
		for srvName, srvDef := range tk.MCPServers {
			if srvName == "gz-ia" {
				return nil, fmt.Errorf("el servidor MCP 'gz-ia' está reservado para uso interno y no puede ser definido por un toolkit")
			}
			if existingDef, conflict := composed.MCPServers[srvName]; conflict {
				if !reflect.DeepEqual(existingDef, srvDef) {
					return nil, fmt.Errorf("colisión de servidores MCP en tooling: el servidor '%s' está definido con configuraciones distintas", srvName)
				}
			}
			composed.MCPServers[srvName] = srvDef
		}

		// 6. Variables de Entorno
		for k, v := range tk.Env {
			if existingVal, conflict := composed.Env[k]; conflict && existingVal != v {
				return nil, fmt.Errorf("colisión de variables de entorno en tooling: variable '%s' con valores distintos", k)
			}
			composed.Env[k] = v
		}
	}

	return composed, nil
}

// RegisterToolsInKernel registra las herramientas descubiertas en el microkernel Orchy.
func (s *toolingService) RegisterToolsInKernel(ctx context.Context, kernel *orchy.Kernel, tooling *ComposedTooling, workDir string) error {
	if kernel == nil || tooling == nil {
		return nil
	}

	for _, decl := range tooling.Tools {
		cmdTool := NewCommandTool(decl, workDir)
		_, err := kernel.RegisterTool(cmdTool, tools.ToolProxyOptions{
			MaxConsecutiveFailures: 5,
		})
		if err != nil {
			return fmt.Errorf("error registrando herramienta '%s' en Orchy: %w", decl.Name, err)
		}
	}
	return nil
}

// ProjectIntoWorktree materializa las capacidades del tooling en el directorio del worktree
// registrando un manifiesto exacto de qué archivos fueron creados y cuáles originales fueron modificados.
func (s *toolingService) ProjectIntoWorktree(ctx context.Context, targetDir string, composed *ComposedTooling, sessionID string, baseDir ...string) error {
	if composed == nil || targetDir == "" {
		return nil
	}

	var base string
	if len(baseDir) > 0 && baseDir[0] != "" {
		base = baseDir[0]
	}

	manifest := workspace.NewManifest(sessionID)
	var prevManifest *workspace.Manifest
	if base != "" {
		if loaded, err := workspace.LoadManifest(base, sessionID); err == nil && loaded != nil {
			prevManifest = loaded
			for k, v := range loaded.OriginalFiles {
				manifest.OriginalFiles[k] = v
			}
			for _, cf := range loaded.CreatedFiles {
				manifest.CreatedFiles = append(manifest.CreatedFiles, cf)
			}
		}
	}

	// 1. Proyectar Skills en .agents/skills/
	if len(composed.SkillPaths) > 0 {
		skillsDir := filepath.Join(targetDir, ".agents", "skills")
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			return fmt.Errorf("error creando directorio de skills %s: %w", skillsDir, err)
		}

		for skillName, srcPath := range composed.SkillPaths {
			relPath := filepath.Join(".agents", "skills", skillName)
			dstPath := filepath.Join(targetDir, relPath)

			if fi, statErr := os.Lstat(dstPath); statErr == nil {
				// Ya existía en target
				isAlreadyCreated := false
				if prevManifest != nil {
					for _, cf := range prevManifest.CreatedFiles {
						if cf == relPath {
							isAlreadyCreated = true
							break
						}
					}
				}
				if !isAlreadyCreated {
					// Skill original del repositorio del usuario: respaldar antes de tocar
					if fi.IsDir() {
						_ = filepath.Walk(dstPath, func(path string, info os.FileInfo, err error) error {
							if err != nil || info.IsDir() {
								return nil
							}
							subRel, rErr := filepath.Rel(targetDir, path)
							if rErr == nil {
								if _, backed := manifest.OriginalFiles[subRel]; !backed {
									if data, rErr := os.ReadFile(path); rErr == nil {
										manifest.OriginalFiles[subRel] = string(data)
									}
								}
							}
							return nil
						})
					} else {
						if _, backed := manifest.OriginalFiles[relPath]; !backed {
							if data, rErr := os.ReadFile(dstPath); rErr == nil {
								manifest.OriginalFiles[relPath] = string(data)
							}
						}
					}
				}
				if err := os.RemoveAll(dstPath); err != nil {
					return fmt.Errorf("error limpiando skill previa en %s: %w", dstPath, err)
				}
			} else {
				alreadyCreated := false
				for _, cf := range manifest.CreatedFiles {
					if cf == relPath {
						alreadyCreated = true
						break
					}
				}
				if !alreadyCreated {
					manifest.CreatedFiles = append(manifest.CreatedFiles, relPath)
				}
			}

			if err := os.Symlink(srcPath, dstPath); err != nil {
				if err := copyDir(srcPath, dstPath); err != nil {
					return fmt.Errorf("error copiando skill %s: %w", skillName, err)
				}
			}

			// Tracking de hash para revertir limpiamente
			_ = filepath.Walk(dstPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				subRel, rErr := filepath.Rel(targetDir, path)
				if rErr == nil {
					if data, rErr := os.ReadFile(path); rErr == nil {
						manifest.ProjectedHash[subRel] = workspace.HashBytes(data)
					}
				}
				return nil
			})
			if linkTarget, err := os.Readlink(dstPath); err == nil {
				manifest.ProjectedHash[relPath] = workspace.HashBytes([]byte(linkTarget))
			}
		}
	}

	// 2. Proyectar Reglas en .agents/rules/
	if len(composed.RulesFiles) > 0 {
		rulesDir := filepath.Join(targetDir, ".agents", "rules")
		if err := os.MkdirAll(rulesDir, 0755); err != nil {
			return fmt.Errorf("error creando directorio de reglas %s: %w", rulesDir, err)
		}

		for ruleFilename, srcPath := range composed.RulesFiles {
			relPath := filepath.Join(".agents", "rules", ruleFilename)
			dstPath := filepath.Join(targetDir, relPath)
			if _, ok := manifest.OriginalFiles[relPath]; !ok {
				if origBytes, err := os.ReadFile(dstPath); err == nil {
					manifest.OriginalFiles[relPath] = string(origBytes)
				} else {
					manifest.CreatedFiles = append(manifest.CreatedFiles, relPath)
				}
			}
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("error copiando regla %s: %w", ruleFilename, err)
			}
			data, err := os.ReadFile(dstPath)
			if err != nil {
				return fmt.Errorf("error leyendo regla proyectada %s: %w", ruleFilename, err)
			}
			manifest.ProjectedHash[relPath] = workspace.HashBytes(data)
		}
	}

	// 3. Proyectar directivas de toolkits individuales (ej. TOOLKIT_COMMON-AGENTS.md)
	for targetFilename, srcPath := range composed.AgentsFiles {
		dstPath := filepath.Join(targetDir, targetFilename)
		if _, ok := manifest.OriginalFiles[targetFilename]; !ok {
			if origBytes, err := os.ReadFile(dstPath); err == nil {
				manifest.OriginalFiles[targetFilename] = string(origBytes)
			} else {
				manifest.CreatedFiles = append(manifest.CreatedFiles, targetFilename)
			}
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("error copiando directiva %s: %w", targetFilename, err)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			return fmt.Errorf("error leyendo directiva proyectada %s: %w", targetFilename, err)
		}
		manifest.ProjectedHash[targetFilename] = workspace.HashBytes(data)
	}

	// 4. Redactar el AGENTS.md maestro unificado preservando reglas previas del proyecto si existían
	masterPath := filepath.Join(targetDir, "AGENTS.md")
	masterContent := generateMasterAgentsMarkdown(composed, sessionID)

	var origContent string
	hasOriginal := false
	if prevManifest != nil {
		if prevOrig, ok := prevManifest.OriginalFiles["AGENTS.md"]; ok {
			origContent = prevOrig
			hasOriginal = true
		}
	}
	if !hasOriginal {
		if origBytes, err := os.ReadFile(masterPath); err == nil {
			origContent = string(origBytes)
			hasOriginal = true
			manifest.OriginalFiles["AGENTS.md"] = origContent
		} else {
			alreadyCreated := false
			for _, cf := range manifest.CreatedFiles {
				if cf == "AGENTS.md" {
					alreadyCreated = true
					break
				}
			}
			if !alreadyCreated {
				manifest.CreatedFiles = append(manifest.CreatedFiles, "AGENTS.md")
			}
		}
	}

	var fullContent string
	if hasOriginal && strings.TrimSpace(origContent) != "" {
		fullContent = fmt.Sprintf("# 📌 Reglas Originales del Proyecto\n\n%s\n\n---\n\n%s", strings.TrimSpace(origContent), masterContent)
	} else {
		fullContent = masterContent
	}

	if err := os.WriteFile(masterPath, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("error escribiendo AGENTS.md maestro: %w", err)
	}
	manifest.ProjectedHash["AGENTS.md"] = workspace.HashBytes([]byte(fullContent))

	// 5. Configurar servidores MCP integrando gz-ia y servidores del proyecto/toolkits
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"gz-ia": map[string]any{
				"command": "gz-ia",
				"args": []string{
					"mcp",
					"--session", sessionID,
					"--tooling", s.loader.ToolingDir(),
				},
			},
		},
	}
	if len(composed.MCPServers) > 0 {
		serversMap := mcpConfig["mcpServers"].(map[string]any)
		for k, v := range composed.MCPServers {
			if k != "gz-ia" {
				serversMap[k] = v
			}
		}
	}

	mergeAndWriteMCP := func(relPath string) error {
		dstPath := filepath.Join(targetDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("error creando directorio para %s: %w", relPath, err)
		}

		var targetMap map[string]any
		var origBytes []byte
		var readErr error

		if prevManifest != nil {
			if prevOrig, ok := prevManifest.OriginalFiles[relPath]; ok {
				origBytes = []byte(prevOrig)
			} else {
				origBytes, readErr = os.ReadFile(dstPath)
			}
		} else {
			origBytes, readErr = os.ReadFile(dstPath)
		}

		if readErr == nil && len(origBytes) > 0 {
			if _, ok := manifest.OriginalFiles[relPath]; !ok {
				manifest.OriginalFiles[relPath] = string(origBytes)
			}
			if err := json.Unmarshal(origBytes, &targetMap); err != nil {
				targetMap = make(map[string]any)
			}
		} else {
			alreadyTracked := false
			for _, cf := range manifest.CreatedFiles {
				if cf == relPath {
					alreadyTracked = true
					break
				}
			}
			if !alreadyTracked {
				manifest.CreatedFiles = append(manifest.CreatedFiles, relPath)
			}
			targetMap = make(map[string]any)
		}

		existingServers, _ := targetMap["mcpServers"].(map[string]any)
		if existingServers == nil {
			existingServers = make(map[string]any)
		}
		newServers := mcpConfig["mcpServers"].(map[string]any)
		for k, v := range newServers {
			existingServers[k] = v
		}
		targetMap["mcpServers"] = existingServers

		mergedData, err := json.MarshalIndent(targetMap, "", "  ")
		if err != nil {
			return fmt.Errorf("error codificando JSON para %s: %w", relPath, err)
		}
		if err := os.WriteFile(dstPath, mergedData, 0644); err != nil {
			return fmt.Errorf("error escribiendo %s: %w", relPath, err)
		}
		manifest.ProjectedHash[relPath] = workspace.HashBytes(mergedData)
		return nil
	}

	if err := mergeAndWriteMCP(filepath.Join(".agents", "mcp_config.json")); err != nil {
		return err
	}
	if err := mergeAndWriteMCP(".mcp.json"); err != nil {
		return err
	}

	// Persistir el manifiesto en baseDir si fue suministrado
	if base != "" {
		if err := workspace.SaveManifest(base, manifest); err != nil {
			return fmt.Errorf("falló al guardar manifiesto de sesión: %w", err)
		}
	}

	return nil
}

func generateMasterAgentsMarkdown(composed *ComposedTooling, sessionID string) string {
	var sb strings.Builder

	sb.WriteString("# Directivas Unificadas de Sesión Agéntica — Gountz IA\n\n")
	sb.WriteString(fmt.Sprintf("> **Sesión ID:** `%s` | **Toolkits Activos:** `%s`\n\n", sessionID, strings.Join(composed.ActiveToolkits, ", ")))

	if len(composed.AgentsFiles) > 0 {
		sb.WriteString("## 📚 Directivas de Dominio y Toolkits\n")
		sb.WriteString("Esta sesión integra los siguientes paquetes de directivas. Consulta y acata las guías de cada archivo:\n\n")
		for filename := range composed.AgentsFiles {
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", filename, filename))
		}
		sb.WriteString("\n")
	}

	if len(composed.RulesFiles) > 0 {
		sb.WriteString("## 📐 Reglas y Estándares de Arquitectura (`.agents/rules/`)\n")
		for ruleFilename := range composed.RulesFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", ruleFilename))
		}
		sb.WriteString("\n")
	}

	if len(composed.Tools) > 0 {
		sb.WriteString("## 🛠️ Herramientas MCP Disponibles (vía Microkernel Orchy)\n")
		for _, t := range composed.Tools {
			desc := t.Description
			if desc == "" {
				desc = "Herramienta de extensión de toolkit"
			}
			sb.WriteString(fmt.Sprintf("- **`%s`**: %s\n", t.Name, desc))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dst, 0755)

	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}
