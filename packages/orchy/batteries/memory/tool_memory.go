package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gz-ia/internal/features/memory"
	"gz-ia/packages/orchy/tools"
)

type SaveInput struct {
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Category  string   `json:"category,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
}

type MemorySaveTool struct {
	svc memory.Service
}

func NewMemorySaveTool(svc memory.Service) *MemorySaveTool {
	return &MemorySaveTool{svc: svc}
}

func (t *MemorySaveTool) Name() string { return "memory_save" }
func (t *MemorySaveTool) Description() string {
	return "Guarda un registro de decisión, regla o conocimiento en la memoria local de gz-ia."
}

func (t *MemorySaveTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para guardar un registro de memoria.",
		Properties: map[string]any{
			"title":      map[string]any{"type": "string", "description": "Título descriptivo del conocimiento o decisión."},
			"content":    map[string]any{"type": "string", "description": "Contenido detallado, código o especificación."},
			"category":   map[string]any{"type": "string", "description": "Categoría (ej. 'decision', 'rule', 'architecture', 'bugfix')."},
			"tags":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Etiquetas para facilitar filtrado."},
			"session_id": map[string]any{"type": "string", "description": "Identificador de la sesión (opcional; si se omite se guarda en el proyecto)."},
		},
		Required: []string{"title", "content"},
	}
}

func (t *MemorySaveTool) Execute(ctx context.Context, input any) (any, error) {
	if t.svc == nil {
		return nil, errors.New("memory service not configured")
	}
	var in SaveInput
	if err := decodeInput(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input for memory_save: %w", err)
	}

	rec, err := t.svc.Save(ctx, memory.Record{
		SessionID: in.SessionID,
		Title:     in.Title,
		Content:   in.Content,
		Category:  in.Category,
		Tags:      in.Tags,
	})
	if err != nil {
		return nil, err
	}
	return rec, nil
}

type SearchInput struct {
	Query      string `json:"query"`
	SessionID  string `json:"session_id,omitempty"`
	GlobalOnly bool   `json:"global_only,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type MemorySearchTool struct {
	svc memory.Service
}

func NewMemorySearchTool(svc memory.Service) *MemorySearchTool {
	return &MemorySearchTool{svc: svc}
}

func (t *MemorySearchTool) Name() string { return "memory_search" }
func (t *MemorySearchTool) Description() string {
	return "Busca decisiones y contexto usando el motor BM25 por relevancia semántica."
}

func (t *MemorySearchTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para buscar memorias por BM25.",
		Properties: map[string]any{
			"query":       map[string]any{"type": "string", "description": "Consulta de búsqueda o términos clave."},
			"session_id":  map[string]any{"type": "string", "description": "ID de sesión para incluir su memoria local (opcional)."},
			"global_only": map[string]any{"type": "boolean", "description": "Buscar exclusivamente en la memoria del proyecto."},
			"limit":       map[string]any{"type": "integer", "description": "Límite de resultados (por defecto 10)."},
		},
		Required: []string{"query"},
	}
}

func (t *MemorySearchTool) Execute(ctx context.Context, input any) (any, error) {
	if t.svc == nil {
		return nil, errors.New("memory service not configured")
	}
	var in SearchInput
	if err := decodeInput(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input for memory_search: %w", err)
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}

	return t.svc.Search(ctx, in.Query, memory.SearchOptions{
		SessionID:  in.SessionID,
		GlobalOnly: in.GlobalOnly,
		Limit:      limit,
	})
}

type ListInput struct {
	SessionID  string `json:"session_id,omitempty"`
	GlobalOnly bool   `json:"global_only,omitempty"`
}

type MemoryListTool struct {
	svc memory.Service
}

func NewMemoryListTool(svc memory.Service) *MemoryListTool {
	return &MemoryListTool{svc: svc}
}

func (t *MemoryListTool) Name() string { return "memory_list" }
func (t *MemoryListTool) Description() string {
	return "Lista todos los registros de memoria guardados en la sesión o el proyecto."
}

func (t *MemoryListTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para listar registros de memoria.",
		Properties: map[string]any{
			"session_id":  map[string]any{"type": "string", "description": "ID de sesión (opcional)."},
			"global_only": map[string]any{"type": "boolean", "description": "Listar exclusivamente memorias del proyecto."},
		},
	}
}

func (t *MemoryListTool) Execute(ctx context.Context, input any) (any, error) {
	if t.svc == nil {
		return nil, errors.New("memory service not configured")
	}
	var in ListInput
	_ = decodeInput(input, &in)

	return t.svc.List(ctx, memory.SearchOptions{
		SessionID:  in.SessionID,
		GlobalOnly: in.GlobalOnly,
	})
}

type ConsolidateInput struct {
	SessionID string `json:"session_id"`
}

type MemoryConsolidateTool struct {
	svc memory.Service
}

func NewMemoryConsolidateTool(svc memory.Service) *MemoryConsolidateTool {
	return &MemoryConsolidateTool{svc: svc}
}

func (t *MemoryConsolidateTool) Name() string { return "memory_consolidate" }
func (t *MemoryConsolidateTool) Description() string {
	return "Promueve y consolida las memorias de una sesión hacia el proyecto global."
}

func (t *MemoryConsolidateTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Kind:        "object",
		Type:        "object",
		Description: "Parámetros para consolidar memoria de sesión a global.",
		Properties: map[string]any{
			"session_id": map[string]any{"type": "string", "description": "Identificador de la sesión a consolidar."},
		},
		Required: []string{"session_id"},
	}
}

func (t *MemoryConsolidateTool) Execute(ctx context.Context, input any) (any, error) {
	if t.svc == nil {
		return nil, errors.New("memory service not configured")
	}
	var in ConsolidateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input for memory_consolidate: %w", err)
	}

	return t.svc.Consolidate(ctx, in.SessionID)
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
