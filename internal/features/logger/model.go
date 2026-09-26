package logger

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Stage representa la etapa en la que se encuentra la acción del agente.
type Stage string

const (
	StageRead    Stage = "READ"
	StagePending Stage = "PENDING"
	StageFinish  Stage = "FINISH"
)

// IsValid verifica si la etapa es válida.
func (s Stage) IsValid() bool {
	switch s {
	case StageRead, StagePending, StageFinish:
		return true
	default:
		return false
	}
}

// Status representa el estado de finalización de una acción o etapa.
type Status string

const (
	StatusOK     Status = "OK"
	StatusFailed Status = "FAILED"
)

// IsValid verifica si el estado es válido.
func (s Status) IsValid() bool {
	switch s {
	case StatusOK, StatusFailed:
		return true
	default:
		return false
	}
}

// StatusPtr retorna un puntero al valor de Status provisto.
func StatusPtr(s Status) *Status {
	return &s
}

// Event modela un evento de observabilidad y trazabilidad emitido por un agente o el harness.
type Event struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id"`
	AgentID    string         `json:"agent_id,omitempty"`
	Role       string         `json:"role,omitempty"`
	Action     string         `json:"action"`
	Stage      Stage          `json:"stage"`
	Status     *Status        `json:"status"` // Serializa exactamente null, "OK" o "FAILED"
	Timestamp  time.Time      `json:"timestamp"`
	DurationMs *int64         `json:"duration_ms,omitempty"`
	Error      string         `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// GenerateEventID genera un identificador único aleatorio para eventos.
func GenerateEventID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:12]
	}
	return hex.EncodeToString(b)
}

// Validate verifica que el evento cumpla con las reglas obligatorias del modelo.
func (e *Event) Validate() error {
	if e == nil {
		return errors.New("evento no puede ser nil")
	}
	if e.SessionID == "" {
		return errors.New("session_id es obligatorio")
	}
	if !e.Stage.IsValid() {
		return fmt.Errorf("etapa inválida: '%s' (válidos: READ, PENDING, FINISH)", e.Stage)
	}
	if e.Status != nil && !e.Status.IsValid() {
		return fmt.Errorf("estado inválido: '%s' (válidos: OK, FAILED)", *e.Status)
	}
	return nil
}
