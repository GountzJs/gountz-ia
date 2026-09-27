package tooling

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gz-ia/packages/orchy/tools"
)

// DefaultToolingDir retorna la ruta por defecto del directorio global de tooling: ~/.config/gz-ia/tooling
func DefaultToolingDir() string {
	if env := os.Getenv("GZ_TOOLING_DIR"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "gz-ia-tooling")
	}
	return filepath.Join(home, ".config", "gz-ia", "tooling")
}

// Loader se encarga de parsear config.json, descubrir toolkits y skills en disco y generar scaffolding.
type Loader struct {
	toolingDir string
	projectDir string
}

// NewLoader crea una nueva instancia de Loader con soporte global y de proyecto.
func NewLoader(toolingDir string, projectDir ...string) *Loader {
	if toolingDir == "" {
		toolingDir = DefaultToolingDir()
	}
	projDir := ""
	if len(projectDir) > 0 && projectDir[0] != "" {
		projDir = projectDir[0]
	} else if cwd, err := os.Getwd(); err == nil {
		projDir = cwd
	}
	return &Loader{
		toolingDir: toolingDir,
		projectDir: projDir,
	}
}

// ToolingDir retorna el directorio base global configurado.
func (l *Loader) ToolingDir() string {
	return l.toolingDir
}

// ProjectDir retorna el directorio raíz del proyecto configurado.
func (l *Loader) ProjectDir() string {
	return l.projectDir
}

// LoadConfig lee y deserializa config.json global y del proyecto. Si no existe ninguno, retorna una configuración vacía.
// Si existe config.json o .harness/tooling/config.json en el proyecto, combina y sobrescribe los presets locales en cfg.Presets.
func (l *Loader) LoadConfig() (*Config, error) {
	var cfg *Config
	var globalPath string

	if l.toolingDir != "" {
		globalPath = filepath.Join(l.toolingDir, "config.json")
		globalCfg, err := parseConfigFile(globalPath)
		if err != nil {
			return nil, err
		}
		cfg = globalCfg
	}

	if cfg == nil {
		cfg = &Config{
			Version: 1,
			Presets: []Preset{},
		}
	}

	if l.projectDir != "" {
		candidates := []string{
			filepath.Join(l.projectDir, "config.json"),
			filepath.Join(l.projectDir, ".harness", "tooling", "config.json"),
		}
		for _, cand := range candidates {
			if globalPath != "" && cand == globalPath {
				continue
			}
			localCfg, err := parseConfigFile(cand)
			if err != nil {
				return nil, err
			}
			if localCfg != nil {
				if cfg.Version == 1 && localCfg.Version > 0 {
					cfg.Version = localCfg.Version
				}
				cfg.Presets = mergePresets(cfg.Presets, localCfg.Presets)
				break
			}
		}
	}

	if cfg.Presets == nil {
		cfg.Presets = []Preset{}
	}

	return cfg, nil
}

// parseConfigFile lee y deserializa un archivo config.json si existe. Si no existe, retorna (nil, nil).
func parseConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error leyendo config.json en %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parseando config.json en %s: %w", path, err)
	}

	if len(cfg.Presets) == 0 && len(cfg.Perfiles) > 0 {
		cfg.Presets = cfg.Perfiles
	}

	return &cfg, nil
}

// mergePresets combina dos listas de presets. Si un preset en overrides coincide en nombre
// (case-insensitive) con uno en base, prevalece el de overrides. Los nuevos se agregan al final.
func mergePresets(base, overrides []Preset) []Preset {
	if len(overrides) == 0 {
		return base
	}
	if len(base) == 0 {
		return overrides
	}

	result := make([]Preset, len(base))
	copy(result, base)

	indexMap := make(map[string]int, len(base))
	for i, p := range result {
		indexMap[strings.ToLower(strings.TrimSpace(p.Name))] = i
	}

	for _, p := range overrides {
		key := strings.ToLower(strings.TrimSpace(p.Name))
		if idx, exists := indexMap[key]; exists {
			result[idx] = p
		} else {
			result = append(result, p)
			indexMap[key] = len(result) - 1
		}
	}

	return result
}

// LoadToolkit escanea y carga un toolkit por su ID buscando primero en el proyecto y luego globalmente.
func (l *Loader) LoadToolkit(id string) (*Toolkit, error) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return nil, fmt.Errorf("el ID del toolkit no puede estar vacío")
	}

	var candidatePaths []struct {
		path  string
		scope string
	}
	if l.projectDir != "" {
		candidatePaths = append(candidatePaths,
			struct{ path, scope string }{path: filepath.Join(l.projectDir, ".gz-ia", "toolkits", cleanID), scope: "project"},
			struct{ path, scope string }{path: filepath.Join(l.projectDir, "toolkits", cleanID), scope: "project"},
			struct{ path, scope string }{path: filepath.Join(l.projectDir, ".harness", "toolkits", cleanID), scope: "project"},
		)
	}
	candidatePaths = append(candidatePaths,
		struct{ path, scope string }{path: filepath.Join(l.toolingDir, "toolkits", cleanID), scope: "global"},
		struct{ path, scope string }{path: filepath.Join(l.toolingDir, cleanID), scope: "global"},
	)

	for _, c := range candidatePaths {
		fi, err := os.Stat(c.path)
		if err == nil && fi.IsDir() {
			return l.loadToolkitFromDir(cleanID, c.path, c.scope)
		}
	}

	return nil, fmt.Errorf("toolkit '%s' no encontrado en el proyecto (%s/.gz-ia/toolkits o toolkits) ni globalmente (%s/toolkits)", cleanID, l.projectDir, l.toolingDir)
}

// loadToolkitFromDir analiza un directorio específico y construye el Toolkit.
func (l *Loader) loadToolkitFromDir(id, tkPath, scope string) (*Toolkit, error) {
	tk := &Toolkit{
		ID:         id,
		Path:       tkPath,
		Scope:      scope,
		RulesPaths: make(map[string]string),
		SkillPaths: make(map[string]string),
		Tools:      []DeclaredTool{},
		MCPServers: make(map[string]any),
		Env:        make(map[string]string),
	}

	// 1. Cargar metadatos desde toolkit.json (si existe)
	toolkitJSONPath := filepath.Join(tkPath, "toolkit.json")
	if data, err := os.ReadFile(toolkitJSONPath); err == nil {
		var meta struct {
			ID          string            `json:"id"`
			Description string            `json:"description"`
			MCPServers  map[string]any    `json:"mcp_servers"`
			MCPServers2 map[string]any    `json:"mcpServers"`
			Env         map[string]string `json:"env"`
		}
		if err := json.Unmarshal(data, &meta); err == nil {
			if meta.Description != "" {
				tk.Description = meta.Description
			}
			if len(meta.MCPServers) > 0 {
				tk.MCPServers = meta.MCPServers
			} else if len(meta.MCPServers2) > 0 {
				tk.MCPServers = meta.MCPServers2
			}
			if len(meta.Env) > 0 {
				tk.Env = meta.Env
			}
		}
	}

	// 2. Detectar AGENTS.md
	agentsPath := filepath.Join(tkPath, "AGENTS.md")
	if fi, err := os.Stat(agentsPath); err == nil && !fi.IsDir() {
		tk.AgentsPath = agentsPath
	}

	// 3. Escanear rules/
	rulesDir := filepath.Join(tkPath, "rules")
	if entries, err := os.ReadDir(rulesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				tk.RulesPaths[e.Name()] = filepath.Join(rulesDir, e.Name())
			}
		}
	}

	// 4. Escanear skills/
	skillsDir := filepath.Join(tkPath, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				skillPath := filepath.Join(skillsDir, e.Name())
				if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err == nil {
					tk.SkillPaths[e.Name()] = skillPath
				}
			}
		}
	}

	// 5. Cargar tools.json declarativo
	toolsJSONPath := filepath.Join(tkPath, "tools.json")
	if data, err := os.ReadFile(toolsJSONPath); err == nil {
		var declared []DeclaredTool
		if err := json.Unmarshal(data, &declared); err == nil {
			tk.Tools = append(tk.Tools, declared...)
		}
	}

	// 6. Escanear scripts ejecutables en tools/
	toolsDir := filepath.Join(tkPath, "tools")
	if entries, err := os.ReadDir(toolsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				scriptPath := filepath.Join(toolsDir, e.Name())
				name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				alreadyDeclared := false
				for _, d := range tk.Tools {
					if d.Name == name {
						alreadyDeclared = true
						break
					}
				}
				if !alreadyDeclared {
					tk.Tools = append(tk.Tools, DeclaredTool{
						Name:        name,
						Description: fmt.Sprintf("Herramienta ejecutada desde %s", e.Name()),
						ScriptPath:  scriptPath,
						Schema: tools.ToolSchema{
							Kind: "object",
						},
					})
				}
			}
		}
	}

	return tk, nil
}

// ListToolkits descubre todos los toolkits disponibles en directorios globales y del proyecto.
func (l *Loader) ListToolkits() ([]Toolkit, error) {
	toolkitsMap := make(map[string]Toolkit)

	var searchDirs = []struct {
		path  string
		scope string
	}{
		// Global primero
		{path: filepath.Join(l.toolingDir, "toolkits"), scope: "global"},
		{path: l.toolingDir, scope: "global"},
		// Proyecto después para precedencia local
		{path: filepath.Join(l.projectDir, ".gz-ia", "toolkits"), scope: "project"},
		{path: filepath.Join(l.projectDir, "toolkits"), scope: "project"},
		{path: filepath.Join(l.projectDir, ".harness", "toolkits"), scope: "project"},
	}

	for _, sd := range searchDirs {
		if sd.path == "" {
			continue
		}
		entries, err := os.ReadDir(sd.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			// Ignorar subdirectorio "toolkits" si estamos en l.toolingDir
			if sd.path == l.toolingDir && entry.Name() == "toolkits" {
				continue
			}

			tkDir := filepath.Join(sd.path, entry.Name())
			if !isToolkitDir(tkDir) {
				continue
			}

			tk, err := l.loadToolkitFromDir(entry.Name(), tkDir, sd.scope)
			if err != nil {
				continue
			}
			toolkitsMap[tk.ID] = *tk
		}
	}

	result := make([]Toolkit, 0, len(toolkitsMap))
	for _, tk := range toolkitsMap {
		result = append(result, tk)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// ListSkills descubre todas las habilidades (.skills/skills) dentro de toolkits y catálogos.
func (l *Loader) ListSkills() ([]SkillInfo, error) {
	toolkits, err := l.ListToolkits()
	if err != nil {
		return nil, err
	}

	skillsMap := make(map[string]SkillInfo)
	for _, tk := range toolkits {
		for skillName, skillPath := range tk.SkillPaths {
			desc := extractSkillDescription(skillPath)
			cat := extractCategoryFromName(skillName)
			key := fmt.Sprintf("%s:%s", tk.ID, skillName)
			skillsMap[key] = SkillInfo{
				Name:        skillName,
				Category:    cat,
				Description: desc,
				Path:        skillPath,
				Scope:       tk.Scope,
				ToolkitID:   tk.ID,
			}
		}
	}

	// Verificar si existen skills independientes en .harness/skills o tooling/skills
	extraDirs := []struct {
		path  string
		scope string
	}{
		{path: filepath.Join(l.toolingDir, "skills"), scope: "global"},
		{path: filepath.Join(l.projectDir, ".harness", "skills"), scope: "project"},
		{path: filepath.Join(l.projectDir, "skills"), scope: "project"},
	}
	for _, ed := range extraDirs {
		if ed.path == "" {
			continue
		}
		entries, err := os.ReadDir(ed.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillDir := filepath.Join(ed.path, e.Name())
			if _, statErr := os.Stat(filepath.Join(skillDir, "SKILL.md")); statErr == nil {
				key := fmt.Sprintf("standalone:%s", e.Name())
				if _, exists := skillsMap[key]; !exists {
					skillsMap[key] = SkillInfo{
						Name:        e.Name(),
						Category:    extractCategoryFromName(e.Name()),
						Description: extractSkillDescription(skillDir),
						Path:        skillDir,
						Scope:       ed.scope,
					}
				}
			}
		}
	}

	result := make([]SkillInfo, 0, len(skillsMap))
	for _, sk := range skillsMap {
		result = append(result, sk)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ToolkitID < result[j].ToolkitID
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// CreateToolkitScaffold inicializa un nuevo toolkit con la estructura estándar:
// AGENTS.md, rules/example.md, skills/example/SKILL.md, tools.json, toolkit.json
func (l *Loader) CreateToolkitScaffold(req CreateToolkitRequest) (*Toolkit, error) {
	cleanID := strings.TrimSpace(req.ID)
	if cleanID == "" {
		return nil, fmt.Errorf("el ID del toolkit no puede estar vacío")
	}

	var baseDir string
	scope := "project"
	if req.Global {
		scope = "global"
		baseDir = filepath.Join(l.toolingDir, "toolkits", cleanID)
	} else {
		if l.projectDir == "" {
			return nil, fmt.Errorf("directorio del proyecto no definido para crear toolkit local")
		}
		gziaToolkitsDir := filepath.Join(l.projectDir, ".gz-ia", "toolkits")
		toolkitsDir := filepath.Join(l.projectDir, "toolkits")
		harnessToolkitsDir := filepath.Join(l.projectDir, ".harness", "toolkits")
		if fi, err := os.Stat(gziaToolkitsDir); err == nil && fi.IsDir() {
			baseDir = filepath.Join(gziaToolkitsDir, cleanID)
		} else if fi, err := os.Stat(toolkitsDir); err == nil && fi.IsDir() {
			baseDir = filepath.Join(toolkitsDir, cleanID)
		} else if fi, err := os.Stat(harnessToolkitsDir); err == nil && fi.IsDir() {
			baseDir = filepath.Join(harnessToolkitsDir, cleanID)
		} else {
			baseDir = filepath.Join(gziaToolkitsDir, cleanID)
		}
	}

	if fi, err := os.Stat(baseDir); err == nil && fi.IsDir() {
		return nil, fmt.Errorf("el toolkit '%s' ya existe en %s", cleanID, baseDir)
	}

	rulesDir := filepath.Join(baseDir, "rules")
	skillDir := filepath.Join(baseDir, "skills", "example")
	toolsDir := filepath.Join(baseDir, "tools")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de reglas: %w", err)
	}
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de skills: %w", err)
	}
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de tools: %w", err)
	}

	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = fmt.Sprintf("Toolkit modular %s", cleanID)
	}

	// 1. toolkit.json
	tkMeta := map[string]any{
		"id":          cleanID,
		"description": desc,
		"env":         req.Env,
		"mcp_servers": map[string]any{},
	}
	if req.Env == nil {
		tkMeta["env"] = map[string]string{}
	}
	tkData, _ := json.MarshalIndent(tkMeta, "", "  ")
	if err := os.WriteFile(filepath.Join(baseDir, "toolkit.json"), tkData, 0644); err != nil {
		return nil, fmt.Errorf("error escribiendo toolkit.json: %w", err)
	}

	// 2. AGENTS.md
	agentsContent := fmt.Sprintf("# Directivas Agénticas — %s\n\n%s\n\n## Convenciones y Guías\n- Define aquí las instrucciones maestras de este toolkit.\n", cleanID, desc)
	if err := os.WriteFile(filepath.Join(baseDir, "AGENTS.md"), []byte(agentsContent), 0644); err != nil {
		return nil, fmt.Errorf("error escribiendo AGENTS.md: %w", err)
	}

	// 3. rules/example.md
	ruleContent := fmt.Sprintf("# Regla de Arquitectura — %s\n\n1. Sigue las directivas arquitectónicas definidas para %s.\n", cleanID, cleanID)
	if err := os.WriteFile(filepath.Join(rulesDir, "example.md"), []byte(ruleContent), 0644); err != nil {
		return nil, fmt.Errorf("error escribiendo rules/example.md: %w", err)
	}

	// 4. skills/example/SKILL.md
	skillContent := fmt.Sprintf("---\nname: example\ndescription: Habilidad de ejemplo para %s\n---\n\n# Habilidad de Ejemplo\n\nInstrucciones paso a paso para ejecutar esta habilidad.\n", cleanID)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		return nil, fmt.Errorf("error escribiendo skills/example/SKILL.md: %w", err)
	}

	// 5. tools.json
	toolsData := []DeclaredTool{
		{
			Name:        "example_tool",
			Description: fmt.Sprintf("Herramienta de ejemplo para el toolkit %s", cleanID),
			Command:     "echo '{\"status\": \"ok\"}'",
		},
	}
	toolsBytes, _ := json.MarshalIndent(toolsData, "", "  ")
	if err := os.WriteFile(filepath.Join(baseDir, "tools.json"), toolsBytes, 0644); err != nil {
		return nil, fmt.Errorf("error escribiendo tools.json: %w", err)
	}

	return l.loadToolkitFromDir(cleanID, baseDir, scope)
}

func isToolkitDir(dir string) bool {
	checks := []string{
		"toolkit.json",
		"perfil.json",
		"profile.json",
		"AGENTS.md",
		"tools.json",
		"rules",
		"skills",
		"tools",
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return true
		}
	}
	return false
}

func extractSkillDescription(skillDir string) string {
	skillFile := filepath.Join(skillDir, "SKILL.md")
	f, err := os.Open(skillFile)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter && strings.HasPrefix(line, "description:") {
			desc := strings.TrimPrefix(line, "description:")
			return strings.Trim(strings.TrimSpace(desc), "\"'")
		}
		if !inFrontmatter && line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

func extractCategoryFromName(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		return parts[0]
	}
	return "general"
}
