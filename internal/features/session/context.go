package session

import (
	"time"

	"gz-ia/internal/features/logger"
)

// SessionContext representa el estado resumido y la trazabilidad de una sesión
// para permitir handoff y reconstrucción de contexto entre agentes u orquestadores.
type SessionContext struct {
	SessionID     string         `json:"session_id"`
	Provider      string         `json:"provider"`
	Branch        string         `json:"branch"`
	Toolkits      []string       `json:"toolkits,omitempty"`
	InitialPrompt string         `json:"initial_prompt,omitempty"`
	WorkingDir    string         `json:"working_dir,omitempty"`
	Status        string         `json:"status"`
	StartedAt     time.Time      `json:"started_at"`
	FinishedAt    *time.Time     `json:"finished_at,omitempty"`
	DurationMs    int64          `json:"duration_ms,omitempty"`
	Events        []logger.Event `json:"events,omitempty"`
	FilesChanged  string         `json:"files_changed,omitempty"`
}
