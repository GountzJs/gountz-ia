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

// Service define el contrato de operaciones para el ecosistema de tooling y perfiles modulares.
type Service interface {
	ListProfiles(ctx context.Context) ([]ProfileConfig, error)
	GetProfile(ctx context.Context, name string) (*ProfileConfig, error)
	ComposeToolkits(ctx context.Context, toolkitIDs []string) (*ComposedTooling, error)
	RegisterToolsInKernel(ctx context.Context, kernel *orchy.Kernel, tooling *ComposedTooling, workDir string) error
	ProjectIntoWorktree(ctx context.Context, targetDir string, composed *ComposedTooling, sessionID string, baseDir ...string) error
	ToolingDir() string
}

type toolingService struct {
	loader *Loader
}

// NewService crea un nuevo servicio de tooling con el loader configurado.
func NewService(toolingDir ...string) Service {
	dir := ""
	if len(toolingDir) > 0 && toolingDir[0] != "" {
		dir = toolingDir[0]
	}
	return &toolingService{
		loader: NewLoader(dir),
	}
}

func (s *toolingService) ToolingDir() string {
	return s.loader.ToolingDir()
}

// ListProfiles retorna la lista de perfiles declarados en config.json.
func (s *toolingService) ListProfiles(ctx context.Context) ([]ProfileConfig, error) {
	cfg, err := s.loader.LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.Perfiles, nil
}

// GetProfile busca un perfil por nombre exacto o normalizado.
func (s *toolingService) GetProfile(ctx context.Context, name string) (*ProfileConfig, error) {
	clean := strings.TrimSpace(strings.ToLower(name))
	profiles, err := s.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range profiles {
		if strings.ToLower(p.Name) == clean {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("perfil de tooling '%s' no encontrado en %s", name, s.loader.ToolingDir())
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

	manifest := workspace.NewManifest(sessionID)

	// 1. Proyectar Skills en .agents/skills/
	if len(composed.SkillPaths) > 0 {
		skillsDir := filepath.Join(targetDir, ".agents", "skills")
		_ = os.MkdirAll(skillsDir, 0755)

		for skillName, srcPath := range composed.SkillPaths {
			relPath := filepath.Join(".agents", "skills", skillName)
			dstPath := filepath.Join(targetDir, relPath)
			if _, statErr := os.Lstat(dstPath); os.IsNotExist(statErr) {
				manifest.CreatedFiles = append(manifest.CreatedFiles, relPath)
			}
			_ = os.Remove(dstPath)
			if err := os.Symlink(srcPath, dstPath); err != nil {
				_ = copyDir(srcPath, dstPath)
			}
		}
	}

	// 2. Proyectar Reglas en .agents/rules/
	if len(composed.RulesFiles) > 0 {
		rulesDir := filepath.Join(targetDir, ".agents", "rules")
		_ = os.MkdirAll(rulesDir, 0755)

		for ruleFilename, srcPath := range composed.RulesFiles {
			relPath := filepath.Join(".agents", "rules", ruleFilename)
			dstPath := filepath.Join(targetDir, relPath)
			if origBytes, err := os.ReadFile(dstPath); err == nil {
				manifest.OriginalFiles[relPath] = string(origBytes)
			} else {
				manifest.CreatedFiles = append(manifest.CreatedFiles, relPath)
			}
			_ = copyFile(srcPath, dstPath)
			if data, err := os.ReadFile(dstPath); err == nil {
				manifest.ProjectedHash[relPath] = workspace.HashBytes(data)
			}
		}
	}

	// 3. Proyectar directivas de toolkits individuales (ej. TOOLKIT_COMMON-AGENTS.md)
	for targetFilename, srcPath := range composed.AgentsFiles {
		dstPath := filepath.Join(targetDir, targetFilename)
		if origBytes, err := os.ReadFile(dstPath); err == nil {
			manifest.OriginalFiles[targetFilename] = string(origBytes)
		} else {
			manifest.CreatedFiles = append(manifest.CreatedFiles, targetFilename)
		}
		_ = copyFile(srcPath, dstPath)
		if data, err := os.ReadFile(dstPath); err == nil {
			manifest.ProjectedHash[targetFilename] = workspace.HashBytes(data)
		}
	}

	// 4. Redactar el AGENTS.md maestro unificado preservando reglas previas del proyecto si existían
	masterPath := filepath.Join(targetDir, "AGENTS.md")
	masterContent := generateMasterAgentsMarkdown(composed, sessionID)
	if origBytes, err := os.ReadFile(masterPath); err == nil {
		manifest.OriginalFiles["AGENTS.md"] = string(origBytes)
		fullContent := fmt.Sprintf("# 📌 Reglas Originales del Proyecto\n\n%s\n\n---\n\n%s", strings.TrimSpace(string(origBytes)), masterContent)
		_ = os.WriteFile(masterPath, []byte(fullContent), 0644)
		manifest.ProjectedHash["AGENTS.md"] = workspace.HashBytes([]byte(fullContent))
	} else {
		manifest.CreatedFiles = append(manifest.CreatedFiles, "AGENTS.md")
		_ = os.WriteFile(masterPath, []byte(masterContent), 0644)
		manifest.ProjectedHash["AGENTS.md"] = workspace.HashBytes([]byte(masterContent))
	}

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
			serversMap[k] = v
		}
	}

	mergeAndWriteMCP := func(relPath string) {
		dstPath := filepath.Join(targetDir, relPath)
		_ = os.MkdirAll(filepath.Dir(dstPath), 0755)

		var targetMap map[string]any
		if origBytes, err := os.ReadFile(dstPath); err == nil {
			manifest.OriginalFiles[relPath] = string(origBytes)
			if err := json.Unmarshal(origBytes, &targetMap); err != nil {
				targetMap = make(map[string]any)
			}
		} else {
			manifest.CreatedFiles = append(manifest.CreatedFiles, relPath)
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
		if err == nil {
			_ = os.WriteFile(dstPath, mergedData, 0644)
			manifest.ProjectedHash[relPath] = workspace.HashBytes(mergedData)
		}
	}

	mergeAndWriteMCP(filepath.Join(".agents", "mcp_config.json"))
	mergeAndWriteMCP(".mcp.json")

	// Persistir el manifiesto en baseDir si fue suministrado
	if len(baseDir) > 0 && baseDir[0] != "" {
		_ = workspace.SaveManifest(baseDir[0], manifest)
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
