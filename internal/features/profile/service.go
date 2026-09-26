package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Service define el contrato del caso de uso de gestión y activación de perfiles agénticos.
type Service interface {
	ListProfiles(ctx context.Context) ([]Profile, error)
	GetProfile(ctx context.Context, name string) (*Profile, error)
	ListSkills(ctx context.Context) ([]Skill, error)
	GetSkill(ctx context.Context, name string) (*Skill, error)
	Compose(ctx context.Context, profileNames []string) (*ComposedProfile, error)
	Project(ctx context.Context, targetDir string, composed *ComposedProfile, sessionID string, driverID string) error
	CreateProfile(ctx context.Context, name string, description string, agentsFile string, global bool) (*Profile, error)
	GlobalDir() string
	ProjectDir() string
}

type profileService struct {
	store     Store
	composer  *Composer
	projector *Projector
}

// NewService crea un nuevo servicio de perfiles con el store provisto o por defecto.
func NewService(opts ...Option) Service {
	store := NewFileStore(opts...)
	return &profileService{
		store:     store,
		composer:  NewComposer(store),
		projector: NewProjector(),
	}
}

// NewServiceWithStore permite inyectar un Store personalizado (útil para pruebas unitarias).
func NewServiceWithStore(store Store) Service {
	return &profileService{
		store:     store,
		composer:  NewComposer(store),
		projector: NewProjector(),
	}
}

func (s *profileService) GlobalDir() string {
	return s.store.GlobalDir()
}

func (s *profileService) ProjectDir() string {
	return s.store.ProjectDir()
}

func (s *profileService) ListProfiles(ctx context.Context) ([]Profile, error) {
	return s.store.ListProfiles()
}

func (s *profileService) GetProfile(ctx context.Context, name string) (*Profile, error) {
	return s.store.GetProfile(name)
}

func (s *profileService) ListSkills(ctx context.Context) ([]Skill, error) {
	return s.store.ListSkills()
}

func (s *profileService) GetSkill(ctx context.Context, name string) (*Skill, error) {
	return s.store.GetSkill(name)
}

func (s *profileService) Compose(ctx context.Context, profileNames []string) (*ComposedProfile, error) {
	return s.composer.Compose(profileNames)
}

func (s *profileService) Project(ctx context.Context, targetDir string, composed *ComposedProfile, sessionID string, driverID string) error {
	return s.projector.Project(ctx, targetDir, composed, sessionID, driverID)
}

// CreateProfile crea un nuevo perfil agéntico estructurado con perfil.json y plantilla de directivas.
func (s *profileService) CreateProfile(ctx context.Context, name string, description string, agentsFile string, global bool) (*Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("el nombre del perfil no puede estar vacío")
	}

	var baseProfilesDir string
	var scope string
	if global {
		baseProfilesDir = filepath.Join(s.store.GlobalDir(), "profiles")
		scope = "global"
	} else {
		baseProfilesDir = filepath.Join(s.store.ProjectDir(), ".harness", "profiles")
		scope = "project"
	}

	profileDir := filepath.Join(baseProfilesDir, name)
	if fi, err := os.Stat(profileDir); err == nil && fi.IsDir() {
		return nil, fmt.Errorf("el perfil '%s' ya existe en '%s'", name, profileDir)
	}

	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return nil, fmt.Errorf("error al crear directorio para el perfil '%s': %w", profileDir, err)
	}

	if agentsFile == "" {
		upper := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		agentsFile = fmt.Sprintf("%s-AGENTS.md", upper)
	}

	p := &Profile{
		Name:        name,
		Description: description,
		AgentsFile:  agentsFile,
		Skills:      []string{},
		MCPServers:  make(map[string]any),
		Env:         make(map[string]string),
		Directory:   profileDir,
		AgentsPath:  filepath.Join(profileDir, agentsFile),
		Scope:       scope,
	}

	// 1. Escribir perfil.json
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(profileDir, "perfil.json"), data, 0644); err != nil {
		return nil, fmt.Errorf("error al escribir perfil.json: %w", err)
	}

	// 2. Escribir plantilla de *-AGENTS.md
	templateContent := fmt.Sprintf(`# Directivas Especializadas: Perfil %s

%s

## Reglas de Codificación y Arquitectura
1. [Escribe aquí las reglas específicas que el agente debe seguir al usar este perfil]
2. [Ejemplo: patrones de diseño, restricciones de dependencias, estilos de código]

## Flujos Recomendados
- [Procedimientos paso a paso para tareas de este dominio]
`, strings.ToUpper(name), description)

	if err := os.WriteFile(p.AgentsPath, []byte(templateContent), 0644); err != nil {
		return nil, fmt.Errorf("error al escribir '%s': %w", p.AgentsPath, err)
	}

	return p, nil
}
