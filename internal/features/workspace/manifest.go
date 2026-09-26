package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Manifest registra con exactitud qué archivos fueron creados o modificados
// por la proyección agéntica en el worktree, junto con sus hashes y contenidos originales.
type Manifest struct {
	SessionID     string            `json:"session_id"`
	CreatedFiles  []string          `json:"created_files"`  // Rutas relativas creadas que no existían
	OriginalFiles map[string]string `json:"original_files"` // Ruta relativa -> contenido previo a la proyección
	ProjectedHash map[string]string `json:"projected_hash"` // Ruta relativa -> sha256 de lo que proyectó gz-ia
}

// NewManifest inicializa un manifiesto vacío para una sesión.
func NewManifest(sessionID string) *Manifest {
	return &Manifest{
		SessionID:     sessionID,
		CreatedFiles:  []string{},
		OriginalFiles: make(map[string]string),
		ProjectedHash: make(map[string]string),
	}
}

// HashBytes calcula el hash SHA-256 en formato hexadecimal.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ManifestPath retorna la ruta de persistencia del manifiesto fuera del worktree.
func ManifestPath(baseDir string, sessionID string) string {
	return filepath.Join(baseDir, ".harness", "sessions", sessionID+".manifest.json")
}

// SaveManifest persiste el manifiesto de la sesión atómicamente en disco.
func SaveManifest(baseDir string, m *Manifest) error {
	if m == nil || m.SessionID == "" {
		return nil
	}
	p := ManifestPath(baseDir, m.SessionID)
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// LoadManifest carga el manifiesto de la sesión si existe.
func LoadManifest(baseDir string, sessionID string) (*Manifest, error) {
	p := ManifestPath(baseDir, sessionID)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
