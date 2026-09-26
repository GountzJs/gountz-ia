package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SessionStatus representa el estado actual de una sesión.
type SessionStatus string

const (
	StatusRunning   SessionStatus = "running"
	StatusCompleted SessionStatus = "completed"
	StatusFailed    SessionStatus = "failed"
	StatusKilled    SessionStatus = "killed"
)

// SessionRecord modela la metadata persistida de una sesión en .harness/sessions/.
type SessionRecord struct {
	ID              string          `json:"id"`
	PID             int             `json:"pid"`
	Provider        string          `json:"provider"`
	Status          SessionStatus   `json:"status"`
	PermissionLevel PermissionLevel `json:"permission_level"`
	WorkingDir      string          `json:"working_dir"`
	InitialPrompt   string          `json:"initial_prompt,omitempty"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	DurationMs      int64           `json:"duration_ms,omitempty"`
	ExitCode        int             `json:"exit_code"`
	IsIsolated      bool            `json:"is_isolated"`
	WorktreeDir     string          `json:"worktree_dir,omitempty"`
	BranchName      string          `json:"branch_name,omitempty"`
	Profiles        []string        `json:"profiles,omitempty"`
}

// GenerateID genera un identificador aleatorio único de 8 caracteres hexadecimales.
func GenerateID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	}
	return hex.EncodeToString(b)
}

// Store define el contrato para persistir y consultar sesiones.
type Store interface {
	Save(s *SessionRecord) error
	Get(id string) (*SessionRecord, error)
	List() ([]SessionRecord, error)
	Delete(id string) error
}

// FileStore implementa Store guardando archivos JSON en una carpeta local (.harness/sessions).
type FileStore struct {
	baseDir string
}

// NewFileStore crea una nueva instancia de FileStore en la ruta dada.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{baseDir: baseDir}
}

// DefaultFileStore crea un FileStore dentro de .harness/sessions en el directorio actual.
func DefaultFileStore(workDir string) *FileStore {
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	return NewFileStore(filepath.Join(workDir, ".harness", "sessions"))
}

func (f *FileStore) ensureDir() error {
	return os.MkdirAll(f.baseDir, 0755)
}

func (f *FileStore) sessionPath(id string) string {
	return filepath.Join(f.baseDir, id+".json")
}

// Save persiste un registro de sesión de forma atómica.
func (f *FileStore) Save(s *SessionRecord) error {
	if s == nil || s.ID == "" {
		return errors.New("registro de sesión inválido")
	}

	if err := f.ensureDir(); err != nil {
		return fmt.Errorf("error al crear directorio de sesiones: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("error al serializar registro de sesión: %w", err)
	}

	target := f.sessionPath(s.ID)
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("error al escribir archivo temporal de sesión: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("error al renombrar archivo de sesión: %w", err)
	}

	return nil
}

// Get obtiene el registro de una sesión por su ID.
func (f *FileStore) Get(id string) (*SessionRecord, error) {
	path := f.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("sesión '%s' no encontrada", id)
		}
		return nil, fmt.Errorf("error al leer sesión '%s': %w", id, err)
	}

	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("error al deserializar sesión '%s': %w", id, err)
	}

	return &record, nil
}

// List retorna todas las sesiones almacenadas, ordenadas por fecha de inicio descendente.
func (f *FileStore) List() ([]SessionRecord, error) {
	if _, err := os.Stat(f.baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		return nil, fmt.Errorf("error al listar directorio de sesiones: %w", err)
	}

	var records []SessionRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-len(".json")]
		rec, err := f.Get(id)
		if err == nil && rec != nil {
			records = append(records, *rec)
		}
	}

	// Orden descendente por fecha de inicio (más recientes primero)
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	return records, nil
}

// Delete elimina el registro de una sesión.
func (f *FileStore) Delete(id string) error {
	path := f.sessionPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("sesión '%s' no encontrada", id)
		}
		return fmt.Errorf("error al eliminar sesión '%s': %w", id, err)
	}
	return nil
}
