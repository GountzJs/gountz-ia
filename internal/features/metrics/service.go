package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Service define el contrato de casos de uso para obtener métricas y telemetría de sesiones.
type Service interface {
	GetMetrics(ctx context.Context, sessionID string, workDir string) (*SessionMetrics, error)
}

type metricsService struct {
	collector *Collector
}

// sessionMetadata modela los campos mínimos necesarios leídos de .harness/sessions/<id>.json.
type sessionMetadata struct {
	ID          string     `json:"id"`
	WorkingDir  string     `json:"working_dir"`
	WorktreeDir string     `json:"worktree_dir"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	DurationMs  int64      `json:"duration_ms"`
}

// NewService crea un nuevo servicio de métricas con las opciones provistas.
func NewService(opts ...Option) Service {
	return &metricsService{
		collector: NewCollector(opts...),
	}
}

// readSessionMetadata intenta leer el registro de sesión local de .harness/sessions/.
// Busca primero en .harness/sessions/<id>/session.json y aplica fallback a .harness/sessions/<id>.json.
func readSessionMetadata(workDir, sessionID string) *sessionMetadata {
	if sessionID == "" {
		return nil
	}

	searchDirs := []string{workDir}
	if cwd, err := os.Getwd(); err == nil && cwd != workDir {
		searchDirs = append(searchDirs, cwd)
	}

	for _, dir := range searchDirs {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, ".harness", "sessions", sessionID, "session.json")
		data, err := os.ReadFile(path)
		if err != nil && os.IsNotExist(err) {
			path = filepath.Join(dir, ".harness", "sessions", sessionID+".json")
			data, err = os.ReadFile(path)
		}
		if err == nil {
			var meta sessionMetadata
			if json.Unmarshal(data, &meta) == nil {
				return &meta
			}
		}
	}
	return nil
}

// GetMetrics extrae y consolida las métricas de la sesión y todos sus subagentes.
func (s *metricsService) GetMetrics(ctx context.Context, sessionID string, workDir string) (*SessionMetrics, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID no puede estar vacío")
	}

	meta := readSessionMetadata(workDir, sessionID)

	var targetDirs []string
	var startedAt time.Time
	if meta != nil {
		if meta.WorktreeDir != "" {
			targetDirs = append(targetDirs, meta.WorktreeDir)
		}
		if meta.WorkingDir != "" {
			targetDirs = append(targetDirs, meta.WorkingDir)
		}
		startedAt = meta.StartedAt
	}
	if workDir != "" {
		targetDirs = append(targetDirs, workDir)
	}

	var convID string
	var err error
	if !startedAt.IsZero() {
		convID, err = s.collector.FindConversationID(sessionID, targetDirs, startedAt)
	} else {
		convID, err = s.collector.FindConversationID(sessionID, targetDirs)
	}

	if err != nil {
		return nil, fmt.Errorf("error al localizar trazas para la sesión '%s': %w", sessionID, err)
	}

	metrics, err := s.collector.Collect(sessionID, convID)
	if err != nil {
		return nil, fmt.Errorf("error al recolectar métricas del transcript '%s': %w", convID, err)
	}

	// Complementar con metadata de la sesión si los pasos del transcript no tienen tiempos suficientes
	if meta != nil {
		if metrics.StartedAt.IsZero() && !meta.StartedAt.IsZero() {
			metrics.StartedAt = meta.StartedAt
		}
		if metrics.FinishedAt == nil && meta.FinishedAt != nil {
			metrics.FinishedAt = meta.FinishedAt
		}
		if metrics.TotalDuration == 0 && meta.DurationMs > 0 {
			metrics.TotalDuration = time.Duration(meta.DurationMs) * time.Millisecond
		}
	}

	return metrics, nil
}
