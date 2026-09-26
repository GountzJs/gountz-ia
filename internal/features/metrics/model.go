package metrics

import (
	"time"
)

// TokenUsage detalla el consumo y distribución de tokens estimados durante una sesión o subagente.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	ThinkingTokens   int `json:"thinking_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// SubagentMetrics recopila la telemetría y métricas de ejecución de un subagente invocado.
type SubagentMetrics struct {
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	TypeName       string         `json:"type_name"`
	Prompt         string         `json:"prompt"`
	Duration       time.Duration  `json:"duration"`
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	StepsCount     int            `json:"steps_count"`
	Tokens         TokenUsage     `json:"tokens"`
	ToolCalls      map[string]int `json:"tool_calls"`
	Status         string         `json:"status"`
}

// SessionMetrics consolida las métricas globales de una sesión de chat, incluyendo sus subagentes.
type SessionMetrics struct {
	SessionID      string            `json:"session_id"`
	ConversationID string            `json:"conversation_id"`
	TotalDuration  time.Duration     `json:"total_duration"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     *time.Time        `json:"finished_at,omitempty"`
	StepsCount     int               `json:"steps_count"`
	Tokens         TokenUsage        `json:"tokens"`
	ToolCalls      map[string]int    `json:"tool_calls"`
	Subagents      []SubagentMetrics `json:"subagents,omitempty"`
}
