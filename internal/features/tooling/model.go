package tooling

import (
	"gz-ia/packages/orchy/tools"
)

// Config representa el archivo de configuración global ~/.config/gz-ia/tooling/config.json
type Config struct {
	Version  int      `json:"version"`
	Presets  []Preset `json:"presets,omitempty"`
	Perfiles []Preset `json:"perfiles,omitempty"` // Compatibilidad hacia atrás con perfiles
}

// Preset define un preset y los toolkits modulares que unifica.
type Preset struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Toolkits    []string          `json:"toolkits"`
	MCPServers  map[string]any    `json:"mcp_servers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// ProfileConfig alias de compatibilidad para Preset
type ProfileConfig = Preset

// SkillInfo representa una habilidad descubierta en un toolkit o catálogo.
type SkillInfo struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Scope       string `json:"scope"` // "project" o "global"
	ToolkitID   string `json:"toolkit_id,omitempty"`
}

// CreateToolkitRequest define los datos requeridos para inicializar un nuevo toolkit vía scaffolding.
type CreateToolkitRequest struct {
	ID          string            `json:"id"`
	Description string            `json:"description,omitempty"`
	Global      bool              `json:"global,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// DeclaredTool representa una herramienta MCP declarada en tools.json o descubierta en tools/
type DeclaredTool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Command     string           `json:"command,omitempty"`
	ScriptPath  string           `json:"script_path,omitempty"`
	Schema      tools.ToolSchema `json:"schema,omitempty"`
}

// Toolkit representa un paquete modular aislado de capacidades agénticas.
type Toolkit struct {
	ID          string            `json:"id"`
	Description string            `json:"description,omitempty"`
	Path        string            `json:"path"`
	Scope       string            `json:"scope,omitempty"`       // "project" o "global"
	AgentsPath  string            `json:"agents_path,omitempty"` // Ruta a AGENTS.md
	RulesPaths  map[string]string `json:"rules_paths,omitempty"` // filename -> ruta absoluta
	SkillPaths  map[string]string `json:"skill_paths,omitempty"` // skillName -> ruta absoluta
	Tools       []DeclaredTool    `json:"tools,omitempty"`
	MCPServers  map[string]any    `json:"mcp_servers,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// ComposedTooling representa el resultado unificado de múltiples toolkits para una sesión.
type ComposedTooling struct {
	ActiveToolkits []string
	AgentsFiles    map[string]string // nombre destino (ej. "COMMON-AGENTS.md") -> ruta origen
	RulesFiles     map[string]string // nombre destino (ej. "architecture.md") -> ruta origen
	SkillPaths     map[string]string // skillName -> ruta origen
	Tools          []DeclaredTool
	MCPServers     map[string]any
	Env            map[string]string
}
