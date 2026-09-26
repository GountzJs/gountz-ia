package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store define el contrato para persistir y cargar el Vault de forma segura.
type Store interface {
	Load() (*Vault, error)
	Save(v *Vault) error
	Path() string
}

// FileStore implementa Store guardando el Vault en .harness/vault.json con permisos 0600 y reemplazo atómico.
type FileStore struct {
	filePath string
	mu       sync.RWMutex
}

// NewFileStore crea una nueva instancia de FileStore para el directorio raíz del proyecto.
func NewFileStore(projectDir string) *FileStore {
	harnessDir := filepath.Join(projectDir, ".harness")
	return &FileStore{
		filePath: filepath.Join(harnessDir, "vault.json"),
	}
}

// Path retorna la ruta absoluta del archivo vault.json.
func (s *FileStore) Path() string {
	return s.filePath
}

// Load lee el archivo vault.json. Si no existe, retorna una estructura Vault vacía sin error.
func (s *FileStore) Load() (*Vault, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return &Vault{
			Version: 1,
			Env:     make(map[string]string),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error al leer archivo del vault (%s): %w", s.filePath, err)
	}

	var v Vault
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("error decodificando vault.json: %w", err)
	}

	if v.Env == nil {
		v.Env = make(map[string]string)
	}
	return &v, nil
}

// Save persiste el Vault en disco garantizando permisos 0600 y escritura atómica temporal.
func (s *FileStore) Save(v *Vault) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error creando directorio del vault %s: %w", dir, err)
	}

	v.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando datos del vault: %w", err)
	}

	// Escritura atómica segura: archivo temporal en el mismo directorio con permisos 0600
	tmpFile, err := os.CreateTemp(dir, ".vault-tmp-*")
	if err != nil {
		return fmt.Errorf("error creando archivo temporal para vault: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("error escribiendo en archivo temporal del vault: %w", err)
	}

	// Permisos estrictos: solo lectura y escritura para el usuario propietario
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("error aplicando permisos 0600 al vault: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("error guardando de forma atómica el vault: %w", err)
	}

	return nil
}
