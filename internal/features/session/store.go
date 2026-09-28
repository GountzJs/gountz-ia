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
	"strings"
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
	return filepath.Join(f.baseDir, id, "session.json")
}

func (f *FileStore) legacySessionPath(id string) string {
	return filepath.Join(f.baseDir, id+".json")
}

// Save persiste un registro de sesión de forma atómica en f.baseDir/<id>/session.json.
func (f *FileStore) Save(s *SessionRecord) error {
	if s == nil || s.ID == "" {
		return errors.New("registro de sesión inválido")
	}

	sessDir := filepath.Join(f.baseDir, s.ID)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de sesión: %w", err)
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

// Get obtiene el registro de una sesión por su ID, buscando primero en f.baseDir/<id>/session.json
// y aplicando fallback al formato legacy f.baseDir/<id>.json.
func (f *FileStore) Get(id string) (*SessionRecord, error) {
	path := f.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			legacyPath := f.legacySessionPath(id)
			legacyData, legacyErr := os.ReadFile(legacyPath)
			if legacyErr != nil {
				if os.IsNotExist(legacyErr) {
					return nil, fmt.Errorf("sesión '%s' no encontrada", id)
				}
				return nil, fmt.Errorf("error al leer sesión '%s': %w", id, legacyErr)
			}
			data = legacyData
		} else {
			return nil, fmt.Errorf("error al leer sesión '%s': %w", id, err)
		}
	}

	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("error al deserializar sesión '%s': %w", id, err)
	}
	if record.ID == "" {
		return nil, fmt.Errorf("sesión '%s' no es un registro válido", id)
	}

	return &record, nil
}

// List retorna todas las sesiones almacenadas, ordenadas por fecha de inicio descendente.
// Soporta tanto directorios por sesión (<id>/session.json) como archivos planos legacy (<id>.json).
func (f *FileStore) List() ([]SessionRecord, error) {
	if _, err := os.Stat(f.baseDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		return nil, fmt.Errorf("error al listar directorio de sesiones: %w", err)
	}

	var records []SessionRecord
	seen := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			sessPath := filepath.Join(f.baseDir, entry.Name(), "session.json")
			if _, err := os.Stat(sessPath); err == nil {
				rec, err := f.Get(entry.Name())
				if err == nil && rec != nil && rec.ID != "" && !seen[rec.ID] {
					records = append(records, *rec)
					seen[rec.ID] = true
				}
			}
			continue
		}

		if filepath.Ext(entry.Name()) != ".json" || strings.HasSuffix(entry.Name(), ".manifest.json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		if seen[id] {
			continue
		}
		rec, err := f.Get(id)
		if err == nil && rec != nil && rec.ID != "" && !seen[rec.ID] {
			records = append(records, *rec)
			seen[rec.ID] = true
		}
	}

	// Orden descendente por fecha de inicio (más recientes primero)
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	return records, nil
}

// Delete elimina el registro de una sesión, removiendo el directorio de la sesión
// o el archivo plano legacy según corresponda.
func (f *FileStore) Delete(id string) error {
	dirPath := filepath.Join(f.baseDir, id)
	legacyPath := f.legacySessionPath(id)

	dirExists := false
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		dirExists = true
	}
	legacyExists := false
	if info, err := os.Stat(legacyPath); err == nil && !info.IsDir() {
		legacyExists = true
	}

	if !dirExists && !legacyExists {
		return fmt.Errorf("sesión '%s' no encontrada", id)
	}

	if dirExists {
		if err := os.RemoveAll(dirPath); err != nil {
			return fmt.Errorf("error al eliminar directorio de sesión '%s': %w", id, err)
		}
	}
	if legacyExists {
		if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error al eliminar sesión legacy '%s': %w", id, err)
		}
	}

	return nil
}
