package tooling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Loader se encarga de parsear config.json y descubrir toolkits en disco.
type Loader struct {
	toolingDir string
}

// NewLoader crea una nueva instancia de Loader.
func NewLoader(toolingDir string) *Loader {
	if toolingDir == "" {
		toolingDir = DefaultToolingDir()
	}
	return &Loader{toolingDir: toolingDir}
}

// ToolingDir retorna el directorio base configurado.
func (l *Loader) ToolingDir() string {
	return l.toolingDir
}

// LoadConfig lee y deserializa config.json. Si no existe, retorna una configuración vacía.
func (l *Loader) LoadConfig() (*Config, error) {
	configPath := filepath.Join(l.toolingDir, "config.json")
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return &Config{
			Version:  1,
			Perfiles: []ProfileConfig{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error leyendo config.json en %s: %w", configPath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("error parseando config.json en %s: %w", configPath, err)
	}

	return &cfg, nil
}

// LoadToolkit escanea y carga un toolkit por su ID desde <toolingDir>/toolkits/<id> o <toolingDir>/<id>.
func (l *Loader) LoadToolkit(id string) (*Toolkit, error) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return nil, fmt.Errorf("el ID del toolkit no puede estar vacío")
	}

	// 1. Probar en <toolingDir>/toolkits/<id>
	tkPath := filepath.Join(l.toolingDir, "toolkits", cleanID)
	fi, err := os.Stat(tkPath)
	if err != nil || !fi.IsDir() {
		// 2. Probar directamente en <toolingDir>/<id>
		tkPath = filepath.Join(l.toolingDir, cleanID)
		fi, err = os.Stat(tkPath)
		if err != nil || !fi.IsDir() {
			return nil, fmt.Errorf("toolkit '%s' no encontrado en %s/toolkits/%s", cleanID, l.toolingDir, cleanID)
		}
	}

	tk := &Toolkit{
		ID:         cleanID,
		Path:       tkPath,
		RulesPaths: make(map[string]string),
		SkillPaths: make(map[string]string),
		Tools:      []DeclaredTool{},
	}

	// 1. Detectar AGENTS.md
	agentsPath := filepath.Join(tkPath, "AGENTS.md")
	if fi, err := os.Stat(agentsPath); err == nil && !fi.IsDir() {
		tk.AgentsPath = agentsPath
	}

	// 2. Escanear rules/
	rulesDir := filepath.Join(tkPath, "rules")
	if entries, err := os.ReadDir(rulesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				tk.RulesPaths[e.Name()] = filepath.Join(rulesDir, e.Name())
			}
		}
	}

	// 3. Escanear skills/
	skillsDir := filepath.Join(tkPath, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				skillPath := filepath.Join(skillsDir, e.Name())
				// Validar que contenga SKILL.md
				if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err == nil {
					tk.SkillPaths[e.Name()] = skillPath
				}
			}
		}
	}

	// 4. Cargar tools.json declarativo
	toolsJSONPath := filepath.Join(tkPath, "tools.json")
	if data, err := os.ReadFile(toolsJSONPath); err == nil {
		var declared []DeclaredTool
		if err := json.Unmarshal(data, &declared); err == nil {
			tk.Tools = append(tk.Tools, declared...)
		}
	}

	// 5. Escanear scripts ejecutables en tools/
	toolsDir := filepath.Join(tkPath, "tools")
	if entries, err := os.ReadDir(toolsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				scriptPath := filepath.Join(toolsDir, e.Name())
				name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				// Si no fue declarada en tools.json, agregarla automáticamente
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
