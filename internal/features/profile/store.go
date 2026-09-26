package profile

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store define el contrato para el descubrimiento y persistencia de perfiles y skills.
type Store interface {
	ListProfiles() ([]Profile, error)
	GetProfile(name string) (*Profile, error)
	ListSkills() ([]Skill, error)
	GetSkill(name string) (*Skill, error)
	GlobalDir() string
	ProjectDir() string
}

// FileStore implementa Store buscando en directorios globales (~/.config/gz-ia) y locales del proyecto (.harness).
type FileStore struct {
	globalDir  string
	projectDir string
}

// Option permite configurar FileStore.
type Option func(*FileStore)

// WithGlobalDir sobreescribe la ruta del directorio global de configuración.
func WithGlobalDir(dir string) Option {
	return func(s *FileStore) {
		s.globalDir = dir
	}
}

// WithProjectDir sobreescribe la ruta del directorio del proyecto.
func WithProjectDir(dir string) Option {
	return func(s *FileStore) {
		s.projectDir = dir
	}
}

// NewFileStore crea una nueva instancia de FileStore con opciones.
func NewFileStore(opts ...Option) *FileStore {
	s := &FileStore{}
	for _, opt := range opts {
		opt(s)
	}

	if s.globalDir == "" {
		if envDir := os.Getenv("GZ_CONFIG_DIR"); envDir != "" {
			s.globalDir = envDir
		} else if home, err := os.UserHomeDir(); err == nil {
			s.globalDir = filepath.Join(home, ".config", "gz-ia")
		} else {
			s.globalDir = ".gz-ia"
		}
	}

	if s.projectDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			s.projectDir = cwd
		} else {
			s.projectDir = "."
		}
	}

	return s
}

func (s *FileStore) GlobalDir() string {
	return s.globalDir
}

func (s *FileStore) ProjectDir() string {
	return s.projectDir
}

// ListProfiles descubre y carga todos los perfiles disponibles, verificando estrictamente la unicidad de nombres.
func (s *FileStore) ListProfiles() ([]Profile, error) {
	profilesMap := make(map[string]Profile)
	sourcesMap := make(map[string]string) // name -> ruta de origen para detectar colisiones

	var searchDirs = []struct {
		path  string
		scope string
	}{
		{path: filepath.Join(s.projectDir, ".harness", "profiles"), scope: "project"},
		{path: filepath.Join(s.projectDir, "profiles"), scope: "project"},
		{path: filepath.Join(s.globalDir, "profiles"), scope: "global"},
	}

	for _, sd := range searchDirs {
		entries, err := os.ReadDir(sd.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			profileDir := filepath.Join(sd.path, entry.Name())
			p, err := loadProfileFromDir(profileDir, sd.scope)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, err
			}

			// Verificación estricta de colisión
			if existingPath, found := sourcesMap[p.Name]; found {
				return nil, fmt.Errorf("colisión de perfiles detectada: el perfil '%s' está definido en múltiples ubicaciones:\n  - %s\n  - %s\nCada perfil debe tener un nombre único para evitar ambigüedad en la sesión", p.Name, existingPath, p.Directory)
			}

			profilesMap[p.Name] = *p
			sourcesMap[p.Name] = p.Directory
		}
	}

	var result []Profile
	for _, p := range profilesMap {
		result = append(result, p)
	}

	return result, nil
}

// GetProfile busca un perfil por nombre exacto.
func (s *FileStore) GetProfile(name string) (*Profile, error) {
	profiles, err := s.ListProfiles()
	if err != nil {
		return nil, err
	}

	for _, p := range profiles {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("perfil agéntico '%s' no encontrado en el catálogo global ni del proyecto", name)
}

// ListSkills descubre todas las habilidades (.skills / skills) disponibles en los catálogos.
func (s *FileStore) ListSkills() ([]Skill, error) {
	skillsMap := make(map[string]Skill)

	var searchDirs = []struct {
		path  string
		scope string
	}{
		// Globales primero para que el proyecto pueda sobreescribir legítimamente un skill por nombre si lo desea
		{path: filepath.Join(s.globalDir, "skills"), scope: "global"},
		{path: filepath.Join(s.globalDir, ".skills"), scope: "global"},
		{path: filepath.Join(s.projectDir, ".harness", "skills"), scope: "project"},
		{path: filepath.Join(s.projectDir, "skills"), scope: "project"},
		{path: filepath.Join(s.projectDir, ".skills"), scope: "project"},
	}

	for _, sd := range searchDirs {
		entries, err := os.ReadDir(sd.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			skillDir := filepath.Join(sd.path, entry.Name())
			skillName := entry.Name()

			desc := extractSkillDescription(skillDir)
			category := extractCategoryFromName(skillName)

			skillsMap[skillName] = Skill{
				Name:        skillName,
				Category:    category,
				Description: desc,
				Path:        skillDir,
				Scope:       sd.scope,
			}
		}
	}

	var result []Skill
	for _, sk := range skillsMap {
		result = append(result, sk)
	}

	return result, nil
}

// GetSkill busca una skill por nombre.
func (s *FileStore) GetSkill(name string) (*Skill, error) {
	skills, err := s.ListSkills()
	if err != nil {
		return nil, err
	}

	for _, sk := range skills {
		if sk.Name == name {
			return &sk, nil
		}
	}

	return nil, fmt.Errorf("skill '%s' no encontrada en el catálogo de habilidades", name)
}

// loadProfileFromDir lee perfil.json o profile.json dentro de un directorio de perfil.
func loadProfileFromDir(dir string, scope string) (*Profile, error) {
	var jsonPath string
	for _, candidate := range []string{"perfil.json", "profile.json"} {
		p := filepath.Join(dir, candidate)
		if _, err := os.Stat(p); err == nil {
			jsonPath = p
			break
		}
	}

	if jsonPath == "" {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("error al leer '%s': %w", jsonPath, err)
	}

	var raw struct {
		Name        string            `json:"name"`
		Nombre      string            `json:"nombre"`
		Description string            `json:"description"`
		Descripcion string            `json:"descripcion"`
		AgentsFile  string            `json:"agents_file"`
		Skills      []string          `json:"skills"`
		MCPServers  map[string]any    `json:"mcp_servers"`
		MCPServers2 map[string]any    `json:"mcpServers"`
		Env         map[string]string `json:"env"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("formato JSON inválido en '%s': %w", jsonPath, err)
	}

	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = strings.TrimSpace(raw.Nombre)
	}
	if name == "" {
		name = filepath.Base(dir)
	}

	desc := strings.TrimSpace(raw.Description)
	if desc == "" {
		desc = strings.TrimSpace(raw.Descripcion)
	}

	mcpServers := raw.MCPServers
	if len(mcpServers) == 0 && len(raw.MCPServers2) > 0 {
		mcpServers = raw.MCPServers2
	}

	p := &Profile{
		Name:        name,
		Description: desc,
		AgentsFile:  strings.TrimSpace(raw.AgentsFile),
		Skills:      raw.Skills,
		MCPServers:  mcpServers,
		Env:         raw.Env,
		Directory:   dir,
		Scope:       scope,
	}

	// Si no especificó agents_file explícitamente, buscar automáticamente cualquier archivo *-AGENTS.md o AGENTS.md
	p.AgentsPath = resolveAgentsPath(dir, p.AgentsFile)
	if p.AgentsFile == "" && p.AgentsPath != "" {
		p.AgentsFile = filepath.Base(p.AgentsPath)
	}

	return p, nil
}

// resolveAgentsPath busca la ruta física del archivo de directivas en el directorio del perfil.
func resolveAgentsPath(dir string, preferredFile string) string {
	if preferredFile != "" {
		candidate := filepath.Join(dir, preferredFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Buscar patrones comunes: *-AGENTS.md, *agents.md, AGENTS.md
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			lower := strings.ToLower(entry.Name())
			if strings.HasSuffix(lower, "agents.md") || strings.HasSuffix(lower, "agent.md") {
				return filepath.Join(dir, entry.Name())
			}
		}
	}

	return ""
}

// extractSkillDescription extrae la descripción desde SKILL.md o YAML frontmatter.
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
		// Si no hay frontmatter, tomar primer párrafo no vacío que no sea título
		if !inFrontmatter && line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// extractCategoryFromName clasifica por prefijo: "front-angular" -> "front", "data-postgres" -> "data"
func extractCategoryFromName(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) > 1 {
		return parts[0]
	}
	return "general"
}
