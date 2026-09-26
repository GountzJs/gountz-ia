package profile

// Profile representa un perfil agéntico modular configurable mediante perfil.json (o profile.json).
type Profile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	AgentsFile  string            `json:"agents_file,omitempty"` // Nombre del archivo de directivas específico (ej. "FRONT-AGENTS.md")
	Skills      []string          `json:"skills,omitempty"`      // Nombres de skills a activar (ej. ["front-angular", "front-tailwind"])
	MCPServers  map[string]any    `json:"mcp_servers,omitempty"` // Definiciones de servidores MCP
	Env         map[string]string `json:"env,omitempty"`         // Variables de entorno requeridas

	// Metadatos calculados durante el descubrimiento
	Directory  string `json:"directory,omitempty"`   // Ruta absoluta al directorio del perfil
	AgentsPath string `json:"agents_path,omitempty"` // Ruta absoluta al archivo *-AGENTS.md resuelto
	Scope      string `json:"scope,omitempty"`       // "global" o "project"
}

// Skill representa una habilidad modular descubierta en el catálogo centralizado o del proyecto.
type Skill struct {
	Name        string `json:"name"`        // Nombre identificador (ej. "front-angular", "data-postgres")
	Category    string `json:"category"`    // Categoría extraída del prefijo (ej. "front", "data", "back", "devops")
	Description string `json:"description"` // Descripción breve (extraída de SKILL.md si existe)
	Path        string `json:"path"`        // Ruta absoluta a la carpeta contenedora del skill
	Scope       string `json:"scope"`       // "global" o "project"
}

// ComposedProfile es el resultado de la unión, deduplicación y resolución de múltiples perfiles seleccionados.
type ComposedProfile struct {
	ActiveProfiles []string          `json:"active_profiles"`
	Skills         []string          `json:"skills"`
	SkillPaths     map[string]string `json:"skill_paths"`  // skillName -> ruta absoluta origen
	AgentsFiles    map[string]string `json:"agents_files"` // nombre archivo destino (ej. "FRONT-AGENTS.md") -> ruta origen
	MCPServers     map[string]any    `json:"mcp_servers"`  // servidores MCP combinados
	Env            map[string]string `json:"env"`          // variables de entorno combinadas
}
