package worktree

import (
	"context"
	"errors"
	"fmt"

	"gz-ia/internal/features/session"
	"gz-ia/packages/orchy/tools"
)

// GetToolInput defines the input parameters for worktree_get.
type GetToolInput struct {
	SessionID string `json:"session_id"`
	NoCommit  bool   `json:"no_commit,omitempty"`
	Squash    bool   `json:"squash,omitempty"`
}

// GetToolOutput defines the structured response for worktree_get.
type GetToolOutput struct {
	SessionID string   `json:"session_id"`
	Success   bool     `json:"success"`
	Message   string   `json:"message"`
	Files     []string `json:"files"`
}

// WorktreeGetTool merges modifications from a session worktree into the active workspace.
type WorktreeGetTool struct {
	sessionService session.Service
}

// NewWorktreeGetTool constructs a new WorktreeGetTool.
func NewWorktreeGetTool(svc session.Service) *WorktreeGetTool {
	return &WorktreeGetTool{sessionService: svc}
}

// Name returns the MCP tool identifier.
func (t *WorktreeGetTool) Name() string {
	return "worktree_get"
}

// Description returns a human-readable explanation of the tool for agents.
func (t *WorktreeGetTool) Description() string {
	return "Trae e integra las modificaciones producidas en el worktree de sesión hacia el directorio de trabajo activo."
}

// Schema returns the JSON schema definition for the tool parameters.
func (t *WorktreeGetTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para fusionar las modificaciones de un worktree de sesión hacia el directorio activo.",
		Properties: map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Identificador único de la sesión de gz-ia.",
			},
			"no_commit": map[string]any{
				"type":        "boolean",
				"description": "Deja los cambios preparados en el stage del repo principal sin comitear.",
			},
			"squash": map[string]any{
				"type":        "boolean",
				"description": "Condensa los cambios en un único commit sin conservar el historial intermedio.",
			},
		},
		Required: []string{"session_id"},
	}
}

// Execute integrates modifications from the session worktree into the active workspace.
func (t *WorktreeGetTool) Execute(ctx context.Context, input any) (any, error) {
	if t.sessionService == nil {
		return nil, errors.New("session service not configured")
	}

	var params GetToolInput
	if err := decodeInput(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input for worktree_get: %w", err)
	}

	if params.SessionID == "" {
		return nil, errors.New("session_id is required")
	}

	res, err := t.sessionService.Get(ctx, params.SessionID)
	if err != nil {
		return nil, err
	}

	files := res.FilesIntegrated
	if files == nil {
		files = []string{}
	}

	return GetToolOutput{
		SessionID: params.SessionID,
		Success:   true,
		Message:   res.Message,
		Files:     files,
	}, nil
}
