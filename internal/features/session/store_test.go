package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if len(id1) != 8 {
		t.Errorf("longitud esperada 8, obtenida %d (%s)", len(id1), id1)
	}
	if id1 == id2 {
		t.Error("GenerateID() generó dos IDs idénticos consecutivos")
	}
}

func TestFileStore_CRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "harness-store-test-*")
	if err != nil {
		t.Fatalf("error al crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)

	// List vacío
	list, err := store.List()
	if err != nil {
		t.Fatalf("List() en directorio vacío falló: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("se esperaba lista vacía, obtenido %d elementos", len(list))
	}

	// Guardar sesión 1
	now := time.Now().Add(-10 * time.Minute)
	s1 := &SessionRecord{
		ID:              "sess0001",
		PID:             1234,
		Status:          StatusCompleted,
		PermissionLevel: PermissionSupervised,
		WorkingDir:      "/path/1",
		StartedAt:       now,
	}
	if err := store.Save(s1); err != nil {
		t.Fatalf("Save(s1) falló: %v", err)
	}

	// Guardar sesión 2 (más reciente)
	now2 := time.Now()
	s2 := &SessionRecord{
		ID:              "sess0002",
		PID:             5678,
		Status:          StatusRunning,
		PermissionLevel: PermissionAutonomous,
		WorkingDir:      "/path/2",
		StartedAt:       now2,
	}
	if err := store.Save(s2); err != nil {
		t.Fatalf("Save(s2) falló: %v", err)
	}

	// Get sesión 1
	got1, err := store.Get("sess0001")
	if err != nil {
		t.Fatalf("Get('sess0001') falló: %v", err)
	}
	if got1.PID != 1234 || got1.Status != StatusCompleted {
		t.Errorf("datos de sesión 1 no coinciden: %+v", got1)
	}

	// List (debe retornar ordenado por StartedAt desc: s2 primero, luego s1)
	list, err = store.List()
	if err != nil {
		t.Fatalf("List() falló: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 sesiones, obtenidas %d", len(list))
	}
	if list[0].ID != "sess0002" || list[1].ID != "sess0001" {
		t.Errorf("orden de lista incorrecto: primero %s, segundo %s", list[0].ID, list[1].ID)
	}

	// Verificar que el archivo fue guardado en la estructura de directorio sess0001/session.json
	expectedPath := filepath.Join(tmpDir, "sess0001", "session.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("se esperaba archivo de sesión en '%s': %v", expectedPath, err)
	}

	// Delete sesión 1
	if err := store.Delete("sess0001"); err != nil {
		t.Fatalf("Delete('sess0001') falló: %v", err)
	}

	// Verificar que el directorio sess0001 fue eliminado
	if _, err := os.Stat(filepath.Join(tmpDir, "sess0001")); !os.IsNotExist(err) {
		t.Errorf("directorio sess0001 debió ser eliminado")
	}

	// Get después de Delete debe fallar
	_, err = store.Get("sess0001")
	if err == nil {
		t.Error("Get('sess0001') debió fallar tras ser eliminada")
	}

	// Delete inexistente
	if err := store.Delete("inexistente"); err == nil {
		t.Error("Delete('inexistente') debió retornar error")
	}
}

func TestFileStore_LegacyFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "harness-store-legacy-*")
	if err != nil {
		t.Fatalf("error al crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)

	// Crear archivo plano legacy: <tmpDir>/legacy_sess.json
	now := time.Now().Add(-5 * time.Minute)
	legacyJSON := []byte(`{
		"id": "legacy_sess",
		"pid": 9999,
		"status": "completed",
		"permission_level": "supervised",
		"working_dir": "/legacy/path",
		"started_at": "` + now.Format(time.RFC3339Nano) + `"
	}`)
	legacyFile := filepath.Join(tmpDir, "legacy_sess.json")
	if err := os.WriteFile(legacyFile, legacyJSON, 0644); err != nil {
		t.Fatalf("error creando archivo legacy: %v", err)
	}

	// Crear una sesión moderna en subdirectorio
	modernNow := time.Now()
	modern := &SessionRecord{
		ID:              "modern_sess",
		PID:             1111,
		Status:          StatusRunning,
		PermissionLevel: PermissionAutonomous,
		WorkingDir:      "/modern/path",
		StartedAt:       modernNow,
	}
	if err := store.Save(modern); err != nil {
		t.Fatalf("Save(modern) falló: %v", err)
	}

	// 1. Get de sesión legacy
	gotLegacy, err := store.Get("legacy_sess")
	if err != nil {
		t.Fatalf("Get('legacy_sess') falló con fallback: %v", err)
	}
	if gotLegacy.ID != "legacy_sess" || gotLegacy.PID != 9999 {
		t.Errorf("datos de sesión legacy incorrectos: %+v", gotLegacy)
	}

	// 2. List mixto (debe retornar modern_sess primero y luego legacy_sess)
	list, err := store.List()
	if err != nil {
		t.Fatalf("List() falló: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("se esperaban 2 sesiones en list mixto, obtenidas %d", len(list))
	}
	if list[0].ID != "modern_sess" || list[1].ID != "legacy_sess" {
		t.Errorf("orden inesperado: primero %s, segundo %s", list[0].ID, list[1].ID)
	}

	// 3. Delete de sesión legacy
	if err := store.Delete("legacy_sess"); err != nil {
		t.Fatalf("Delete('legacy_sess') falló: %v", err)
	}
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Errorf("archivo legacy debió ser eliminado")
	}
	if _, err := store.Get("legacy_sess"); err == nil {
		t.Error("Get('legacy_sess') debió fallar tras ser eliminada")
	}
}

func TestFileStore_InvalidSave(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "harness-store-invalid-*")
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)
	if err := store.Save(nil); err == nil {
		t.Error("Save(nil) debió retornar error")
	}

	if err := store.Save(&SessionRecord{ID: ""}); err == nil {
		t.Error("Save con ID vacío debió retornar error")
	}
}

func TestDefaultFileStore(t *testing.T) {
	store := DefaultFileStore("/test/workdir")
	expected := filepath.Join("/test/workdir", ".harness", "sessions")
	if store.baseDir != expected {
		t.Errorf("baseDir esperado '%s', obtenido '%s'", expected, store.baseDir)
	}
}

func TestFileStore_IgnoresManifestFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "harness-store-manifest-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)

	// Crear una sesión válida
	s1 := &SessionRecord{
		ID:        "valid_sess",
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}
	if err := store.Save(s1); err != nil {
		t.Fatalf("Save falló: %v", err)
	}

	// Crear un archivo manifest espurio en la misma carpeta
	manifestData := []byte(`{"session_id":"valid_sess","projected_hash":{}}`)
	if err := os.WriteFile(filepath.Join(tmpDir, "valid_sess.manifest.json"), manifestData, 0644); err != nil {
		t.Fatalf("WriteFile manifest falló: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List falló: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("se esperaba 1 sesión, obtenidas %d: %+v", len(list), list)
	}
	if list[0].ID != "valid_sess" {
		t.Errorf("ID esperado 'valid_sess', obtenido '%s'", list[0].ID)
	}
}
