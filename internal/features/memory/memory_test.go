package memory

import (
	"context"
	"os"
	"testing"
)

func TestRecord_Validate(t *testing.T) {
	r := &Record{}
	if err := r.Validate(); err == nil {
		t.Error("esperaba error con registro vacío")
	}

	r.Title = "Título de prueba"
	if err := r.Validate(); err == nil {
		t.Error("esperaba error sin contenido")
	}

	r.Content = "Contenido de prueba"
	if err := r.Validate(); err != nil {
		t.Errorf("no esperaba error con registro válido: %v", err)
	}
}

func TestBM25Search(t *testing.T) {
	records := []Record{
		{ID: "1", Title: "Optimización de base de datos", Content: "Se ajustaron los índices de PostgreSQL."},
		{ID: "2", Title: "Autenticación OAuth", Content: "Configuración de tokens JWT."},
		{ID: "3", Title: "Base de datos y Vault", Content: "Integración de secretos para PostgreSQL.", Tags: []string{"database", "vault"}},
	}

	results := BM25Search(records, "base de datos postgresql", 10)
	if len(results) == 0 {
		t.Fatal("esperaba resultados de búsqueda BM25")
	}

	if results[0].Record.ID != "1" && results[0].Record.ID != "3" {
		t.Errorf("esperaba que el primer resultado fuera ID 1 o 3, obtenido: %s", results[0].Record.ID)
	}
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory-test-*")
	if err != nil {
		t.Fatalf("error creando temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewFileStore(tmpDir)

	// Test Session Memory
	sessRecs := []Record{
		{ID: "m1", Title: "Regla 1", Content: "Contenido 1", Category: "rule"},
	}
	if err := store.SaveSession("sess1", sessRecs); err != nil {
		t.Fatalf("SaveSession falló: %v", err)
	}

	loadedSess, err := store.LoadSession("sess1")
	if err != nil {
		t.Fatalf("LoadSession falló: %v", err)
	}
	if len(loadedSess) != 1 || loadedSess[0].Title != "Regla 1" {
		t.Errorf("LoadSession devolvió datos inesperados: %v", loadedSess)
	}

	// Test Project Memory
	projRecs := []Record{
		{ID: "m2", Title: "Regla Global", Content: "Contenido Global", Category: "rule"},
	}
	if err := store.SaveProject(projRecs); err != nil {
		t.Fatalf("SaveProject falló: %v", err)
	}

	loadedProj, err := store.LoadProject()
	if err != nil {
		t.Fatalf("LoadProject falló: %v", err)
	}
	if len(loadedProj) != 1 || loadedProj[0].Title != "Regla Global" {
		t.Errorf("LoadProject devolvió datos inesperados: %v", loadedProj)
	}
}

func TestMergeRecords(t *testing.T) {
	existing := []Record{
		{ID: "1", Title: "Regla A", Category: "rule", Tags: []string{"go"}},
	}
	incoming := []Record{
		{ID: "2", Title: "Regla A", Category: "rule", Tags: []string{"testing"}},
		{ID: "3", Title: "Regla B", Category: "rule"},
	}

	merged := MergeRecords(existing, incoming)
	if len(merged) != 2 {
		t.Fatalf("esperaba 2 registros deduplicados por título:categoría, obtenidos: %d", len(merged))
	}
}

func TestMemoryService_SaveSearchConsolidate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memory-svc-test-*")
	if err != nil {
		t.Fatalf("error creando temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	svc := NewService(tmpDir)
	ctx := context.Background()

	// Guardar en sesión
	rec, err := svc.Save(ctx, Record{
		SessionID: "sess_test",
		Title:     "Arquitectura Hexagonal",
		Content:   "Desacoplamiento mediante puertos y adaptadores",
		Category:  "architecture",
		Tags:      []string{"design", "go"},
	})
	if err != nil {
		t.Fatalf("Save falló: %v", err)
	}
	if rec.ID == "" {
		t.Error("esperaba ID asignado a la memoria")
	}

	// Buscar en sesión
	results, err := svc.Search(ctx, "adaptadores", SearchOptions{SessionID: "sess_test"})
	if err != nil {
		t.Fatalf("Search falló: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("esperaba encontrar resultado por término 'adaptadores'")
	}

	// Consolidar a global
	consolidated, err := svc.Consolidate(ctx, "sess_test")
	if err != nil {
		t.Fatalf("Consolidate falló: %v", err)
	}
	if len(consolidated) != 1 {
		t.Errorf("esperaba 1 registro consolidado, obtenidos: %d", len(consolidated))
	}

	// Verificar en list global
	globalList, err := svc.List(ctx, SearchOptions{GlobalOnly: true})
	if err != nil {
		t.Fatalf("List global falló: %v", err)
	}
	if len(globalList) != 1 {
		t.Errorf("esperaba 1 registro en list global, obtenidos: %d", len(globalList))
	}
}
