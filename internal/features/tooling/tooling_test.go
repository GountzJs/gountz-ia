package tooling

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gz-ia/packages/orchy"
)

func TestLoader_LoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	loader := NewLoader(tempDir)

	// 1. Inexistente retorna vacío sin error
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() inesperadamente falló: %v", err)
	}
	if len(cfg.Perfiles) != 0 {
		t.Errorf("Se esperaba lista vacía de perfiles")
	}

	// 2. Crear config.json sintético
	configData := `{
		"version": 1,
		"perfiles": [
			{
				"name": "Programador React Native",
				"description": "Desarrollo móvil",
				"toolkits": ["toolkit-common", "toolkit-rn"]
			}
		]
	}`
	_ = os.WriteFile(filepath.Join(tempDir, "config.json"), []byte(configData), 0644)

	cfg, err = loader.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() falló con archivo válido: %v", err)
	}
	if len(cfg.Perfiles) != 1 || cfg.Perfiles[0].Name != "Programador React Native" {
		t.Errorf("Perfil inesperado: %v", cfg.Perfiles)
	}
}

func TestLoader_LoadToolkit(t *testing.T) {
	tempDir := t.TempDir()
	toolkitsDir := filepath.Join(tempDir, "toolkits", "toolkit-rn")
	_ = os.MkdirAll(toolkitsDir, 0755)

	// Crear AGENTS.md
	_ = os.WriteFile(filepath.Join(toolkitsDir, "AGENTS.md"), []byte("# Directivas RN\n"), 0644)

	// Crear rules/
	rulesDir := filepath.Join(toolkitsDir, "rules")
	_ = os.MkdirAll(rulesDir, 0755)
	_ = os.WriteFile(filepath.Join(rulesDir, "mobile-arch.md"), []byte("# Arch rules\n"), 0644)

	// Crear skills/
	skillDir := filepath.Join(toolkitsDir, "skills", "react-native-bridge")
	_ = os.MkdirAll(skillDir, 0755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: bridge\n---\n"), 0644)

	// Crear tools.json
	toolsJSON := `[
		{
			"name": "metro_status",
			"description": "Verifica estado del bundler Metro",
			"command": "echo '{\"running\": true}'"
		}
	]`
	_ = os.WriteFile(filepath.Join(toolkitsDir, "tools.json"), []byte(toolsJSON), 0644)

	loader := NewLoader(tempDir)
	tk, err := loader.LoadToolkit("toolkit-rn")
	if err != nil {
		t.Fatalf("LoadToolkit() falló: %v", err)
	}

	if tk.ID != "toolkit-rn" {
		t.Errorf("ID inesperado: %s", tk.ID)
	}
	if tk.AgentsPath == "" {
		t.Errorf("AGENTS.md no detectado")
	}
	if len(tk.RulesPaths) != 1 || tk.RulesPaths["mobile-arch.md"] == "" {
		t.Errorf("Reglas no detectadas: %v", tk.RulesPaths)
	}
	if len(tk.SkillPaths) != 1 || tk.SkillPaths["react-native-bridge"] == "" {
		t.Errorf("Skills no detectadas: %v", tk.SkillPaths)
	}
	if len(tk.Tools) != 1 || tk.Tools[0].Name != "metro_status" {
		t.Errorf("Tools no detectadas: %v", tk.Tools)
	}
}

func TestCommandTool_Execute(t *testing.T) {
	tool := NewCommandTool(DeclaredTool{
		Name:        "test_echo",
		Description: "Prueba echo",
		Command:     "echo '{\"status\": \"ok\"}'",
	}, "")

	res, err := tool.Execute(context.Background(), map[string]any{"arg": 1})
	if err != nil {
		t.Fatalf("Execute() falló: %v", err)
	}

	m, ok := res.(map[string]any)
	if !ok || m["status"] != "ok" {
		t.Errorf("Resultado inesperado: %v", res)
	}
}

func TestToolingService_ComposeAndProject(t *testing.T) {
	tempToolingDir := t.TempDir()

	// 1. Crear toolkit-a
	tkADir := filepath.Join(tempToolingDir, "toolkits", "toolkit-a")
	_ = os.MkdirAll(filepath.Join(tkADir, "rules"), 0755)
	_ = os.MkdirAll(filepath.Join(tkADir, "skills", "skill-a"), 0755)
	_ = os.WriteFile(filepath.Join(tkADir, "AGENTS.md"), []byte("# Toolkit A\n"), 0644)
	_ = os.WriteFile(filepath.Join(tkADir, "rules", "rule-a.md"), []byte("# Rule A\n"), 0644)
	_ = os.WriteFile(filepath.Join(tkADir, "skills", "skill-a", "SKILL.md"), []byte("# Skill A\n"), 0644)

	// 2. Crear toolkit-b con una herramienta
	tkBDir := filepath.Join(tempToolingDir, "toolkits", "toolkit-b")
	_ = os.MkdirAll(filepath.Join(tkBDir, "rules"), 0755)
	_ = os.WriteFile(filepath.Join(tkBDir, "AGENTS.md"), []byte("# Toolkit B\n"), 0644)
	toolsData := `[{"name": "build_tool", "command": "echo build_ok"}]`
	_ = os.WriteFile(filepath.Join(tkBDir, "tools.json"), []byte(toolsData), 0644)

	svc := NewService(tempToolingDir)
	ctx := context.Background()

	composed, err := svc.ComposeToolkits(ctx, []string{"toolkit-a", "toolkit-b"})
	if err != nil {
		t.Fatalf("ComposeToolkits() falló: %v", err)
	}

	if len(composed.ActiveToolkits) != 2 {
		t.Fatalf("Toolkits activos esperados 2, obtenidos %d", len(composed.ActiveToolkits))
	}
	if len(composed.AgentsFiles) != 2 {
		t.Errorf("Agents files esperados 2, obtenidos %d", len(composed.AgentsFiles))
	}
	if len(composed.Tools) != 1 {
		t.Errorf("Tools esperadas 1, obtenidas %d", len(composed.Tools))
	}

	// 3. Proyectar en worktree temporal
	targetWorktreeDir := t.TempDir()
	sessionID := "test_sess_proj_1"

	err = svc.ProjectIntoWorktree(ctx, targetWorktreeDir, composed, sessionID)
	if err != nil {
		t.Fatalf("ProjectIntoWorktree() falló: %v", err)
	}

	// Validar que AGENTS.md maestro fue escrito y contiene enlaces a los toolkits
	masterAgentsBytes, err := os.ReadFile(filepath.Join(targetWorktreeDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md maestro no fue encontrado: %v", err)
	}
	masterContent := string(masterAgentsBytes)
	if !strings.Contains(masterContent, "TOOLKIT_A-AGENTS.md") || !strings.Contains(masterContent, "TOOLKIT_B-AGENTS.md") {
		t.Errorf("AGENTS.md maestro incompleto: %s", masterContent)
	}

	// Validar que las reglas fueron proyectadas en .agents/rules/
	ruleAPath := filepath.Join(targetWorktreeDir, ".agents", "rules", "rule-a.md")
	if _, err := os.Stat(ruleAPath); os.IsNotExist(err) {
		t.Errorf("Regla rule-a.md no fue proyectada en .agents/rules/")
	}

	// Validar que las skills fueron proyectadas en .agents/skills/
	skillAPath := filepath.Join(targetWorktreeDir, ".agents", "skills", "skill-a", "SKILL.md")
	if _, err := os.Stat(skillAPath); os.IsNotExist(err) {
		t.Errorf("Skill skill-a no fue proyectada en .agents/skills/")
	}

	// Validar que .agents/mcp_config.json y .mcp.json apuntan a gz-ia mcp
	mcpPath := filepath.Join(targetWorktreeDir, ".mcp.json")
	mcpData, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf(".mcp.json no fue generado: %v", err)
	}

	var mcpMap map[string]any
	if err := json.Unmarshal(mcpData, &mcpMap); err != nil {
		t.Fatalf(".mcp.json no contiene JSON válido: %v", err)
	}
	if !strings.Contains(string(mcpData), "gz-ia") || !strings.Contains(string(mcpData), sessionID) {
		t.Errorf(".mcp.json no contiene invocación a gz-ia mcp: %s", string(mcpData))
	}
}

func TestToolingService_RegisterToolsInKernel(t *testing.T) {
	svc := NewService()
	kernel := orchy.NewKernel()

	tooling := &ComposedTooling{
		Tools: []DeclaredTool{
			{
				Name:        "sample_cmd",
				Description: "Comando de prueba",
				Command:     "echo sample",
			},
		},
	}

	err := svc.RegisterToolsInKernel(context.Background(), kernel, tooling, "")
	if err != nil {
		t.Fatalf("RegisterToolsInKernel() falló: %v", err)
	}

	// Verificar introspección de la herramienta registrada
	tool, exists := kernel.GetContext().Tools().Get("sample_cmd")
	if !exists {
		t.Fatalf("Herramienta sample_cmd no encontrada en Orchy")
	}
	if tool.Name() != "sample_cmd" {
		t.Errorf("Nombre de tool inesperado: %s", tool.Name())
	}
}
