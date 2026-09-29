package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	return filepath.Join(baseDir, ".harness", "sessions", sessionID, "manifest.json")
}

// SaveManifest persiste el manifiesto de la sesión atómicamente en disco.
func SaveManifest(baseDir string, m *Manifest) error {
	if m == nil || m.SessionID == "" {
		return nil
	}
	p := ManifestPath(baseDir, m.SessionID)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// LoadManifest carga el manifiesto de la sesión si existe, buscando primero
// en .harness/sessions/<sessionID>/manifest.json y aplicando fallback a .harness/sessions/<sessionID>.manifest.json.
func LoadManifest(baseDir string, sessionID string) (*Manifest, error) {
	p := ManifestPath(baseDir, sessionID)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			legacyPath := filepath.Join(baseDir, ".harness", "sessions", sessionID+".manifest.json")
			legacyData, legacyErr := os.ReadFile(legacyPath)
			if legacyErr != nil {
				return nil, legacyErr
			}
			data = legacyData
		} else {
			return nil, err
		}
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateManifestDelta audita mediante `git status --porcelain` el delta del worktree
// (archivos creados o modificados) y actualiza manifest.json en manifestPath.
func UpdateManifestDelta(manifestPath string, worktreeDir string) error {
	if worktreeDir == "" || manifestPath == "" {
		return nil
	}

	var m Manifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	if m.CreatedFiles == nil {
		m.CreatedFiles = []string{}
	}
	if m.OriginalFiles == nil {
		m.OriginalFiles = make(map[string]string)
	}
	if m.ProjectedHash == nil {
		m.ProjectedHash = make(map[string]string)
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	createdMap := make(map[string]bool)
	for _, f := range m.CreatedFiles {
		createdMap[f] = true
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		relPath := strings.TrimSpace(line[3:])
		if strings.HasPrefix(relPath, "\"") && strings.HasSuffix(relPath, "\"") {
			relPath = strings.Trim(relPath, "\"")
		}

		isCreated := strings.Contains(status, "?") || strings.Contains(status, "A")
		if isCreated {
			if !createdMap[relPath] {
				createdMap[relPath] = true
				m.CreatedFiles = append(m.CreatedFiles, relPath)
			}
		}

		fullPath := filepath.Join(worktreeDir, relPath)
		if data, err := os.ReadFile(fullPath); err == nil {
			m.ProjectedHash[relPath] = HashBytes(data)
		}
	}

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0644)
}

