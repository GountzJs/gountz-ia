package memory

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SearchOptions define los filtros y límites para consultar memorias.
type SearchOptions struct {
	SessionID  string
	GlobalOnly bool
	Limit      int
}

// Service define el contrato del ecosistema de memoria semántica local.
type Service interface {
	Save(ctx context.Context, rec Record) (*Record, error)
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	List(ctx context.Context, opts SearchOptions) ([]Record, error)
	Consolidate(ctx context.Context, sessionID string) ([]Record, error)
}

type memoryService struct {
	store Store
}

// NewService construye un Service inyectando el Store correspondiente.
func NewService(projectDir string, store ...Store) Service {
	var st Store = NewFileStore(projectDir)
	if len(store) > 0 && store[0] != nil {
		st = store[0]
	}
	return &memoryService{store: st}
}

// Save persiste una nueva memoria o actualiza una existente en la sesión o globalmente.
func (s *memoryService) Save(ctx context.Context, rec Record) (*Record, error) {
	if err := rec.Validate(); err != nil {
		return nil, err
	}

	now := time.Now()
	if rec.ID == "" {
		rec.ID = GenerateID()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now

	if rec.SessionID != "" {
		// Guardar en la sesión activa
		records, err := s.store.LoadSession(rec.SessionID)
		if err != nil {
			return nil, err
		}
		records = MergeRecords(records, []Record{rec})
		if err := s.store.SaveSession(rec.SessionID, records); err != nil {
			return nil, err
		}
	} else {
		// Guardar en la memoria global del proyecto
		records, err := s.store.LoadProject()
		if err != nil {
			return nil, err
		}
		records = MergeRecords(records, []Record{rec})
		if err := s.store.SaveProject(records); err != nil {
			return nil, err
		}
	}

	return &rec, nil
}

// Search busca entradas relevantes usando el algoritmo BM25 combinando sesión y proyecto.
func (s *memoryService) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	allRecords, err := s.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	return BM25Search(allRecords, query, opts.Limit), nil
}

// List obtiene los registros de memoria respetando los filtros de ámbito.
func (s *memoryService) List(ctx context.Context, opts SearchOptions) ([]Record, error) {
	var combined []Record

	// 1. Cargar memoria global del proyecto si no se restringe explícitamente a sesión
	projectRecs, err := s.store.LoadProject()
	if err != nil {
		return nil, err
	}
	combined = append(combined, projectRecs...)

	// 2. Cargar memoria de la sesión si fue especificada y no es solo global
	if !opts.GlobalOnly && opts.SessionID != "" {
		sessionRecs, err := s.store.LoadSession(opts.SessionID)
		if err != nil {
			return nil, err
		}
		combined = MergeRecords(combined, sessionRecs)
	}

	return combined, nil
}

// Consolidate promueve los registros de memoria de una sesión hacia el almacén global del proyecto.
func (s *memoryService) Consolidate(ctx context.Context, sessionID string) ([]Record, error) {
	if sessionID == "" {
		return nil, errors.New("session_id es obligatorio para consolidar memoria")
	}

	sessionRecs, err := s.store.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("error cargando memoria de sesión a consolidar: %w", err)
	}
	if len(sessionRecs) == 0 {
		return nil, nil
	}

	projectRecs, err := s.store.LoadProject()
	if err != nil {
		return nil, fmt.Errorf("error cargando memoria de proyecto para consolidar: %w", err)
	}

	consolidated := MergeRecords(projectRecs, sessionRecs)
	if err := s.store.SaveProject(consolidated); err != nil {
		return nil, fmt.Errorf("error guardando memoria de proyecto consolidada: %w", err)
	}

	return sessionRecs, nil
}
