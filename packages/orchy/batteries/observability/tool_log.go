package observability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gz-ia/internal/features/logger"
	"gz-ia/packages/orchy/tools"
)

// LogToolInput define los parámetros de entrada para la herramienta MCP session_log.
type LogToolInput struct {
	Action    string `json:"action"`
	Stage     string `json:"stage,omitempty"`
	Status    string `json:"status,omitempty"`
	Role      string `json:"role,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Duration  *int64 `json:"duration,omitempty"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// LogToolOutput define la respuesta estructurada de la herramienta session_log.
type LogToolOutput struct {
	Success   bool      `json:"success"`
	SessionID string    `json:"session_id"`
	Action    string    `json:"action"`
	Stage     string    `json:"stage"`
	Status    *string   `json:"status,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionLogToolOption define opciones para configurar SessionLogTool.
type SessionLogToolOption func(*SessionLogTool)

// WithDefaultSessionID define el identificador de sesión por defecto si el invocador no lo suministra.
func WithDefaultSessionID(sessionID string) SessionLogToolOption {
	return func(t *SessionLogTool) {
		t.defaultSessionID = sessionID
	}
}

// SessionLogTool registra hitos, etapas y observabilidad de sesión mediante MCP.
type SessionLogTool struct {
	loggerService    logger.Service
	defaultSessionID string
}

// NewSessionLogTool construye una nueva instancia de SessionLogTool.
func NewSessionLogTool(l logger.Service, opts ...SessionLogToolOption) *SessionLogTool {
	t := &SessionLogTool{
		loggerService: l,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name retorna el identificador único de la herramienta MCP.
func (t *SessionLogTool) Name() string {
	return "session_log"
}

// Description retorna la descripción legible para modelos de IA y agentes.
func (t *SessionLogTool) Description() string {
	return "Registra un evento de observabilidad y trazabilidad (etapas READ, PENDING, FINISH) en la sesión activa de gz-ia."
}

// Schema retorna la especificación JSON Schema de los parámetros de session_log.
func (t *SessionLogTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para registrar un evento de observabilidad en el log de sesión de gz-ia.",
		Properties: map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Descripción clara de la acción ejecutada o en curso (requerido).",
			},
			"stage": map[string]any{
				"type":        "string",
				"enum":        []string{"READ", "PENDING", "FINISH"},
				"description": "Etapa del ciclo de vida agéntico: READ (exploración), PENDING (ejecución activa), FINISH (culminación). Predeterminado: PENDING.",
			},
			"status": map[string]any{
				"type":        "string",
				"enum":        []string{"OK", "FAILED", ""},
				"description": "Resultado de la etapa: OK o FAILED (opcional, habitualmente en FINISH).",
			},
			"role": map[string]any{
				"type":        "string",
				"description": "Rol del agente o subagente emisor (ej. 'QA', 'Implementer', 'Orchestrator').",
			},
			"agent": map[string]any{
				"type":        "string",
				"description": "Identificador del agente o subagente emisor (por defecto 'orchestrator').",
			},
			"duration": map[string]any{
				"type":        "integer",
				"description": "Duración de la acción en milisegundos (opcional).",
			},
			"error": map[string]any{
				"type":        "string",
				"description": "Detalle del error si el estado fue FAILED (opcional).",
			},
			"session_id": map[string]any{
				"type":        "string",
				"description": "Identificador de la sesión de gz-ia (opcional si la sesión activa está configurada en el servidor MCP).",
			},
		},
		Required: []string{"action"},
	}
}

// Execute procesa la solicitud de logging y persiste el evento en el servicio de observabilidad.
func (t *SessionLogTool) Execute(ctx context.Context, input any) (any, error) {
	if t.loggerService == nil {
		return nil, errors.New("logger service not configured")
	}

	var params LogToolInput
	if err := decodeInput(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input for session_log: %w", err)
	}

	action := strings.TrimSpace(params.Action)
	if action == "" {
		return nil, errors.New("action is required")
	}

	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		sessionID = t.defaultSessionID
	}
	if sessionID == "" {
		return nil, errors.New("session_id is required")
	}

	stageStr := strings.ToUpper(strings.TrimSpace(params.Stage))
	if stageStr == "" {
		stageStr = string(logger.StagePending)
	}
	stage := logger.Stage(stageStr)
	if !stage.IsValid() {
		return nil, fmt.Errorf("invalid stage '%s' (valid values: READ, PENDING, FINISH)", stageStr)
	}

	var statusPtr *logger.Status
	statusStr := strings.ToUpper(strings.TrimSpace(params.Status))
	if statusStr != "" {
		st := logger.Status(statusStr)
		if !st.IsValid() {
			return nil, fmt.Errorf("invalid status '%s' (valid values: OK, FAILED)", statusStr)
		}
		statusPtr = &st
	}

	agent := strings.TrimSpace(params.Agent)
	if agent == "" {
		agent = "orchestrator"
	}

	evt := &logger.Event{
		SessionID:  sessionID,
		AgentID:    agent,
		Role:       strings.TrimSpace(params.Role),
		Action:     action,
		Stage:      stage,
		Status:     statusPtr,
		DurationMs: params.Duration,
		Error:      strings.TrimSpace(params.Error),
		Timestamp:  time.Now(),
	}

	if err := t.loggerService.Emit(ctx, evt); err != nil {
		return nil, fmt.Errorf("failed to emit session log event: %w", err)
	}

	var statusOut *string
	if statusPtr != nil {
		s := string(*statusPtr)
		statusOut = &s
	}

	return LogToolOutput{
		Success:   true,
		SessionID: evt.SessionID,
		Action:    evt.Action,
		Stage:     string(evt.Stage),
		Status:    statusOut,
		Timestamp: evt.Timestamp,
	}, nil
}

func decodeInput(input any, target any) error {
	if input == nil {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
