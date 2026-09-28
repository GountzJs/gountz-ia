package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store define el contrato de almacenamiento local de memoria.
type Store interface {
	LoadSession(sessionID string) ([]Record, error)
	SaveSession(sessionID string, records []Record) error
	LoadProject() ([]Record, error)
	SaveProject(records []Record) error
}

type fileStore struct {
	projectDir string
	mu         sync.RWMutex
}

// NewFileStore crea un nuevo Store basado en archivos JSON atómicos.
func NewFileStore(projectDir string) Store {
	return &fileStore{projectDir: projectDir}
}

func (s *fileStore) projectMemoryPath() string {
	return filepath.Join(s.projectDir, ".harness", "memory.json")
}

func (s *fileStore) sessionMemoryPath(sessionID string) string {
	return filepath.Join(s.projectDir, ".harness", "sessions", sessionID, "memory.json")
}

func (s *fileStore) LoadSession(sessionID string) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sessionID == "" {
		return nil, nil
	}

	path := s.sessionMemoryPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error leyendo memoria de sesión '%s': %w", sessionID, err)
	}

	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("error parseando memoria de sesión '%s': %w", sessionID, err)
	}
	return records, nil
}

func (s *fileStore) SaveSession(sessionID string, records []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sessionID == "" {
		return nil
	}

	path := s.sessionMemoryPath(sessionID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creando directorio de memoria de sesión: %w", err)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando memoria de sesión: %w", err)
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("error escribiendo memoria de sesión temporal: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("error al guardar memoria de sesión atómicamente: %w", err)
	}

	return nil
}

func (s *fileStore) LoadProject() ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.projectMemoryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error leyendo memoria global del proyecto: %w", err)
	}

	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("error parseando memoria global del proyecto: %w", err)
	}
	return records, nil
}

func (s *fileStore) SaveProject(records []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.projectMemoryPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creando directorio de memoria global: %w", err)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando memoria global: %w", err)
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("error escribiendo memoria global temporal: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("error al guardar memoria global atómicamente: %w", err)
	}

	return nil
}

// MergeRecords unifica dos listas de memoria deduplicando por ID o por par (Title, Category).
func MergeRecords(existing, incoming []Record) []Record {
	byID := make(map[string]Record)
	byTitleCategory := make(map[string]Record)
	var order []string

	addRecord := func(r Record) {
		key := fmt.Sprintf("%s:%s", strings.ToLower(strings.TrimSpace(r.Title)), strings.ToLower(strings.TrimSpace(r.Category)))
		if prev, exists := byTitleCategory[key]; exists {
			r.Tags = mergeTags(prev.Tags, r.Tags)
			r.ID = prev.ID
			if r.CreatedAt.IsZero() {
				r.CreatedAt = prev.CreatedAt
			}
			r.UpdatedAt = time.Now()
		}
		if r.ID == "" {
			r.ID = GenerateID()
		}
		if r.CreatedAt.IsZero() {
			r.CreatedAt = time.Now()
		}
		if r.UpdatedAt.IsZero() {
			r.UpdatedAt = r.CreatedAt
		}

		if _, found := byID[r.ID]; !found {
			order = append(order, r.ID)
		}
		byID[r.ID] = r
		byTitleCategory[key] = r
	}

	for _, r := range existing {
		addRecord(r)
	}
	for _, r := range incoming {
		addRecord(r)
	}

	result := make([]Record, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id])
	}
	return result
}

func mergeTags(t1, t2 []string) []string {
	seen := make(map[string]bool)
	var res []string
	for _, t := range append(t1, t2...) {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm != "" && !seen[norm] {
			seen[norm] = true
			res = append(res, t)
		}
	}
	return res
}
