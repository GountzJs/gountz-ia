package session

import (
	"errors"
	"fmt"
	"os/exec"
)

// Driver define la abstracción para interactuar con diferentes agentes de terminal.
type Driver interface {
	ID() string
	DisplayName() string
	BinaryName() string
	IsAvailable() bool
	InstallHint() string
	BuildArgs(cfg Config) ([]string, error)
}

// lookPath permite interceptar la búsqueda en PATH para pruebas unitarias.
var lookPath = exec.LookPath

// SetLookPathForTesting permite interceptar LookPath durante tests de otros paquetes.
func SetLookPathForTesting(fn func(file string) (string, error)) func() {
	orig := lookPath
	lookPath = fn
	return func() { lookPath = orig }
}

// AgyDriver implementa el driver para Google Antigravity CLI (agy).
type AgyDriver struct{}

func (d *AgyDriver) ID() string          { return "agy" }
func (d *AgyDriver) DisplayName() string { return "Google Antigravity (agy)" }
func (d *AgyDriver) BinaryName() string  { return "agy" }
func (d *AgyDriver) InstallHint() string {
	return "Instala Google Antigravity CLI siguiendo las instrucciones oficiales."
}
func (d *AgyDriver) IsAvailable() bool {
	_, err := lookPath(d.BinaryName())
	return err == nil
}
func (d *AgyDriver) BuildArgs(cfg Config) ([]string, error) {
	if cfg.WorkingDir == "" {
		return nil, errors.New("working directory no puede estar vacío")
	}

	var args []string
	perm := cfg.PermissionLevel
	if perm == "" {
		perm = PermissionSupervised
	}

	switch perm {
	case PermissionReadOnly:
		args = append(args, "--mode", "plan")
	case PermissionSupervised:
		// Modo interactivo estándar [y/N]
	case PermissionAutonomous:
		args = append(args, "--dangerously-skip-permissions")
	default:
		return nil, fmt.Errorf("nivel de permiso desconocido: '%s' (válidos: readonly, supervised, autonomous)", perm)
	}

	if cfg.Resume {
		args = append(args, "--continue")
	}

	if cfg.InitialPrompt != "" {
		args = append(args, "-i", cfg.InitialPrompt)
	}

	return args, nil
}

// ClaudeDriver implementa el driver para Claude Code CLI.
type ClaudeDriver struct{}

func (d *ClaudeDriver) ID() string          { return "claude" }
func (d *ClaudeDriver) DisplayName() string { return "Claude Code (claude)" }
func (d *ClaudeDriver) BinaryName() string  { return "claude" }
func (d *ClaudeDriver) InstallHint() string {
	return "npm install -g @anthropic-ai/claude-code"
}
func (d *ClaudeDriver) IsAvailable() bool {
	_, err := lookPath(d.BinaryName())
	return err == nil
}
func (d *ClaudeDriver) BuildArgs(cfg Config) ([]string, error) {
	if cfg.WorkingDir == "" {
		return nil, errors.New("working directory no puede estar vacío")
	}

	var args []string
	perm := cfg.PermissionLevel
	if perm == "" {
		perm = PermissionSupervised
	}

	switch perm {
	case PermissionReadOnly:
		// Modo seguro interactivo sin auto-aprobación ni flags mutantes
	case PermissionSupervised:
		// Modo interactivo estándar
	case PermissionAutonomous:
		args = append(args, "--dangerously-skip-permissions")
	default:
		return nil, fmt.Errorf("nivel de permiso desconocido: '%s' (válidos: readonly, supervised, autonomous)", perm)
	}

	if cfg.Resume {
		args = append(args, "--resume")
	}

	if cfg.InitialPrompt != "" {
		args = append(args, "-p", cfg.InitialPrompt)
	}

	return args, nil
}

// OpenCodeDriver implementa el driver para OpenCode CLI.
type OpenCodeDriver struct{}

func (d *OpenCodeDriver) ID() string          { return "opencode" }
func (d *OpenCodeDriver) DisplayName() string { return "OpenCode (opencode)" }
func (d *OpenCodeDriver) BinaryName() string  { return "opencode" }
func (d *OpenCodeDriver) InstallHint() string {
	return "curl -fsSL https://opencode.ai/install | bash"
}
func (d *OpenCodeDriver) IsAvailable() bool {
	_, err := lookPath(d.BinaryName())
	return err == nil
}
func (d *OpenCodeDriver) BuildArgs(cfg Config) ([]string, error) {
	if cfg.WorkingDir == "" {
		return nil, errors.New("working directory no puede estar vacío")
	}

	var args []string
	perm := cfg.PermissionLevel
	if perm == "" {
		perm = PermissionSupervised
	}

	switch perm {
	case PermissionReadOnly:
		// Modo seguro interactivo
	case PermissionSupervised:
		// Modo supervisado interactivo estándar
	case PermissionAutonomous:
		args = append(args, "--dangerously-skip-permissions")
	default:
		return nil, fmt.Errorf("nivel de permiso desconocido: '%s' (válidos: readonly, supervised, autonomous)", perm)
	}

	if cfg.Resume {
		args = append(args, "--continue")
	}

	if cfg.InitialPrompt != "" {
		args = append(args, cfg.InitialPrompt)
	}

	return args, nil
}

// PiAgentDriver implementa el driver para Pi Agent CLI.
type PiAgentDriver struct{}

func (d *PiAgentDriver) ID() string          { return "pi-agent" }
func (d *PiAgentDriver) DisplayName() string { return "Pi Agent (pi-agent)" }
func (d *PiAgentDriver) BinaryName() string  { return "pi-agent" }
func (d *PiAgentDriver) InstallHint() string {
	return "Consulta la documentación oficial de Pi Agent para la instalación de su CLI."
}
func (d *PiAgentDriver) IsAvailable() bool {
	_, err := lookPath(d.BinaryName())
	return err == nil
}
func (d *PiAgentDriver) BuildArgs(cfg Config) ([]string, error) {
	if cfg.WorkingDir == "" {
		return nil, errors.New("working directory no puede estar vacío")
	}

	var args []string
	perm := cfg.PermissionLevel
	if perm == "" {
		perm = PermissionSupervised
	}

	switch perm {
	case PermissionReadOnly:
		// Modo solo lectura / seguro
	case PermissionSupervised:
		// Modo supervisado interactivo
	case PermissionAutonomous:
		args = append(args, "--dangerously-skip-permissions")
	default:
		return nil, fmt.Errorf("nivel de permiso desconocido: '%s' (válidos: readonly, supervised, autonomous)", perm)
	}

	if cfg.Resume {
		args = append(args, "--resume")
	}

	if cfg.InitialPrompt != "" {
		args = append(args, "-p", cfg.InitialPrompt)
	}

	return args, nil
}

var driversRegistry = []Driver{
	&AgyDriver{},
	&ClaudeDriver{},
	&OpenCodeDriver{},
	&PiAgentDriver{},
}

// ListDrivers retorna la lista ordenada de drivers soportados (agy, claude, opencode, pi-agent).
func ListDrivers() []Driver {
	res := make([]Driver, len(driversRegistry))
	copy(res, driversRegistry)
	return res
}

// GetDriver retorna el driver por su identificador. Si id es vacío, asume "agy".
func GetDriver(id string) (Driver, error) {
	if id == "" {
		id = "agy"
	}
	for _, d := range driversRegistry {
		if d.ID() == id {
			return d, nil
		}
	}
	return nil, fmt.Errorf("proveedor de agente desconocido: '%s'", id)
}

// FirstAvailableDriver retorna el primer driver disponible en el PATH del sistema, o nil si ninguno está instalado.
func FirstAvailableDriver() Driver {
	for _, d := range driversRegistry {
		if d.IsAvailable() {
			return d
		}
	}
	return nil
}
