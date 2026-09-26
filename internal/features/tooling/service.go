package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gz-ia/packages/orchy"
	"gz-ia/packages/orchy/tools"
)

// Service define el contrato de operaciones para el ecosistema de tooling y perfiles modulares.
type Service interface {
	ListProfiles(ctx context.Context) ([]ProfileConfig, error)
	GetProfile(ctx context.Context, name string) (*ProfileConfig, error)
	ComposeToolkits(ctx context.Context, toolkitIDs []string) (*ComposedTooling, error)
	RegisterToolsInKernel(ctx context.Context, kernel *orchy.Kernel, tooling *ComposedTooling, workDir string) error
	ProjectIntoWorktree(ctx context.Context, targetDir string, composed *ComposedTooling, sessionID string) error
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
			composed.AgentsFiles[targetName] = tk.AgentsPath
		}

		// 2. Reglas Markdown
		for ruleName, rulePath := range tk.RulesPaths {
			composed.RulesFiles[ruleName] = rulePath
		}

		// 3. Skills
		for skillName, skillPath := range tk.SkillPaths {
			composed.SkillPaths[skillName] = skillPath
		}

		// 4. Herramientas MCP
		for _, tool := range tk.Tools {
			if !seenTools[tool.Name] {
				seenTools[tool.Name] = true
				composed.Tools = append(composed.Tools, tool)
			}
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

// ProjectIntoWorktree materializa las capacidades del tooling en el directorio del worktree.
func (s *toolingService) ProjectIntoWorktree(ctx context.Context, targetDir string, composed *ComposedTooling, sessionID string) error {
	if composed == nil || targetDir == "" {
		return nil
	}

	// 1. Proyectar Skills en .agents/skills/
	if len(composed.SkillPaths) > 0 {
		skillsDir := filepath.Join(targetDir, ".agents", "skills")
		_ = os.MkdirAll(skillsDir, 0755)

		for skillName, srcPath := range composed.SkillPaths {
			dstPath := filepath.Join(skillsDir, skillName)
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
			dstPath := filepath.Join(rulesDir, ruleFilename)
			_ = copyFile(srcPath, dstPath)
		}
	}

	// 3. Proyectar directivas de toolkits individuales (ej. TOOLKIT_COMMON-AGENTS.md)
	for targetFilename, srcPath := range composed.AgentsFiles {
		dstPath := filepath.Join(targetDir, targetFilename)
		_ = copyFile(srcPath, dstPath)
	}

	// 4. Redactar el AGENTS.md maestro unificado
	masterContent := generateMasterAgentsMarkdown(composed, sessionID)
	masterPath := filepath.Join(targetDir, "AGENTS.md")
	_ = os.WriteFile(masterPath, []byte(masterContent), 0644)

	// 5. Configurar el servidor MCP de gz-ia apuntando a sí mismo (Stdio)
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

	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err == nil {
		// Para Antigravity (.agents/mcp_config.json)
		agentsDir := filepath.Join(targetDir, ".agents")
		_ = os.MkdirAll(agentsDir, 0755)
		_ = os.WriteFile(filepath.Join(agentsDir, "mcp_config.json"), data, 0644)

		// Para Claude Code (.mcp.json)
		_ = os.WriteFile(filepath.Join(targetDir, ".mcp.json"), data, 0644)
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
