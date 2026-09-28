package workspace_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gz-ia/internal/features/workspace"
)

func TestManifestPath(t *testing.T) {
	baseDir := "/test/workdir"
	sessID := "sess-1234"
	expected := filepath.Join(baseDir, ".harness", "sessions", sessID, "manifest.json")
	got := workspace.ManifestPath(baseDir, sessID)
	if got != expected {
		t.Errorf("ManifestPath esperado '%s', obtenido '%s'", expected, got)
	}
}

func TestManifest_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	sessID := "sess-modern"

	m := workspace.NewManifest(sessID)
	m.CreatedFiles = []string{"file1.txt", "file2.txt"}
	m.OriginalFiles["AGENTS.md"] = "# Original"
	m.ProjectedHash["AGENTS.md"] = workspace.HashBytes([]byte("# Projected"))

	if err := workspace.SaveManifest(tmpDir, m); err != nil {
		t.Fatalf("SaveManifest falló: %v", err)
	}

	// Verificar que el archivo fue escrito en .harness/sessions/sess-modern/manifest.json
	expectedPath := filepath.Join(tmpDir, ".harness", "sessions", sessID, "manifest.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("se esperaba archivo de manifiesto en '%s': %v", expectedPath, err)
	}

	// Cargar manifiesto
	loaded, err := workspace.LoadManifest(tmpDir, sessID)
	if err != nil {
		t.Fatalf("LoadManifest falló: %v", err)
	}
	if loaded.SessionID != sessID {
		t.Errorf("SessionID esperado '%s', obtenido '%s'", sessID, loaded.SessionID)
	}
	if len(loaded.CreatedFiles) != 2 {
		t.Errorf("CreatedFiles esperados 2, obtenidos %d", len(loaded.CreatedFiles))
	}
	if loaded.OriginalFiles["AGENTS.md"] != "# Original" {
		t.Errorf("OriginalFiles no coincide: %v", loaded.OriginalFiles["AGENTS.md"])
	}
}

func TestManifest_LegacyFallback(t *testing.T) {
	tmpDir := t.TempDir()
	sessID := "sess-legacy"

	legacyDir := filepath.Join(tmpDir, ".harness", "sessions")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("error creando directorio: %v", err)
	}

	legacyFile := filepath.Join(legacyDir, sessID+".manifest.json")
	m := workspace.NewManifest(sessID)
	m.CreatedFiles = []string{"legacy.txt"}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if err := os.WriteFile(legacyFile, data, 0644); err != nil {
		t.Fatalf("error escribiendo manifest legacy: %v", err)
	}

	loaded, err := workspace.LoadManifest(tmpDir, sessID)
	if err != nil {
		t.Fatalf("LoadManifest falló en fallback legacy: %v", err)
	}
	if loaded.SessionID != sessID {
		t.Errorf("SessionID esperado '%s', obtenido '%s'", sessID, loaded.SessionID)
	}
	if len(loaded.CreatedFiles) != 1 || loaded.CreatedFiles[0] != "legacy.txt" {
		t.Errorf("CreatedFiles inesperado: %v", loaded.CreatedFiles)
	}
}
