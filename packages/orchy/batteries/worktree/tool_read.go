package worktree

import (
	"context"
	"errors"
	"fmt"

	"gz-ia/internal/features/session"
	"gz-ia/packages/orchy/tools"
)

// ReadToolInput defines the input parameters for worktree_read.
type ReadToolInput struct {
	SessionID string `json:"session_id"`
	StatOnly  bool   `json:"stat_only,omitempty"`
}

// ReadToolOutput defines the structured response for worktree_read.
type ReadToolOutput struct {
	SessionID string `json:"session_id"`
	Diff      string `json:"diff"`
	StatOnly  bool   `json:"stat_only"`
}

// WorktreeReadTool inspects the diff and content of a session's worktree.
type WorktreeReadTool struct {
	sessionService session.Service
}

// NewWorktreeReadTool constructs a new WorktreeReadTool.
func NewWorktreeReadTool(svc session.Service) *WorktreeReadTool {
	return &WorktreeReadTool{sessionService: svc}
}

// Name returns the MCP tool identifier.
func (t *WorktreeReadTool) Name() string {
	return "worktree_read"
}

// Description returns a human-readable explanation of the tool for agents.
func (t *WorktreeReadTool) Description() string {
	return "Inspecciona el contenido y diff actual de un worktree de sesión de gz-ia sin alterar el workspace base."
}

// Schema returns the JSON schema definition for the tool parameters.
func (t *WorktreeReadTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para inspeccionar el diff de un worktree de sesión.",
		Properties: map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Identificador único de la sesión de gz-ia.",
			},
			"stat_only": map[string]any{
				"type":        "boolean",
				"description": "Si es true, muestra únicamente el resumen estadístico de archivos modificados.",
			},
		},
		Required: []string{"session_id"},
	}
}

// Execute performs the diff extraction against the session worktree.
func (t *WorktreeReadTool) Execute(ctx context.Context, input any) (any, error) {
	if t.sessionService == nil {
		return nil, errors.New("session service not configured")
	}

	var params ReadToolInput
	if err := decodeInput(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input for worktree_read: %w", err)
	}

	if params.SessionID == "" {
		return nil, errors.New("session_id is required")
	}

	diffOutput, err := t.sessionService.Read(ctx, params.SessionID, params.StatOnly)
	if err != nil {
		return nil, err
	}

	return ReadToolOutput{
		SessionID: params.SessionID,
		Diff:      diffOutput,
		StatOnly:  params.StatOnly,
	}, nil
}
