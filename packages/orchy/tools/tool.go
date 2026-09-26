package tools

import (
	"context"
)

// Tool represents an executable unit of capability within the microkernel.
type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(ctx context.Context, input any) (any, error)
}

// FuncTool is a lightweight adapter struct that implements Tool via a function.
type FuncTool struct {
	ToolName        string
	ToolDescription string
	ToolSchema      ToolSchema
	Handler         func(ctx context.Context, input any) (any, error)
}

func (f *FuncTool) Name() string {
	return f.ToolName
}

func (f *FuncTool) Description() string {
	return f.ToolDescription
}

func (f *FuncTool) Schema() ToolSchema {
	return f.ToolSchema
}

func (f *FuncTool) Execute(ctx context.Context, input any) (any, error) {
	if f.Handler != nil {
		return f.Handler(ctx, input)
	}
	return nil, nil
}
