package tooling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"gz-ia/packages/orchy/tools"
)

// CommandTool implementa tools.Tool ejecutando un comando de shell o script en el sistema.
type CommandTool struct {
	decl DeclaredTool
	dir  string
}

// NewCommandTool crea una nueva herramienta ejecutable de Orchy a partir de una DeclaredTool.
func NewCommandTool(decl DeclaredTool, workDir string) *CommandTool {
	if decl.Schema.Kind == "" {
		decl.Schema.Kind = "object"
	}
	return &CommandTool{
		decl: decl,
		dir:  workDir,
	}
}

func (t *CommandTool) Name() string {
	return t.decl.Name
}

func (t *CommandTool) Description() string {
	return t.decl.Description
}

func (t *CommandTool) Schema() tools.ToolSchema {
	return t.decl.Schema
}

func (t *CommandTool) Execute(ctx context.Context, input any) (any, error) {
	inputJSON, _ := json.Marshal(input)

	var cmd *exec.Cmd
	if t.decl.ScriptPath != "" {
		cmd = exec.CommandContext(ctx, t.decl.ScriptPath)
	} else if t.decl.Command != "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", t.decl.Command)
	} else {
		return nil, fmt.Errorf("la herramienta '%s' no tiene comando ni script configurado", t.decl.Name)
	}

	if t.dir != "" {
		cmd.Dir = t.dir
	}

	// Pasar los parámetros de entrada por stdin como JSON
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		if errStr != "" {
			return nil, fmt.Errorf("error ejecutando '%s': %w (%s)", t.decl.Name, err, errStr)
		}
		return nil, fmt.Errorf("error ejecutando '%s': %w", t.decl.Name, err)
	}

	// Si la salida es JSON válido, intentar retornarla como objeto estructurado
	var jsonResult any
	if json.Unmarshal([]byte(outStr), &jsonResult) == nil {
		return jsonResult, nil
	}

	return map[string]any{
		"output": outStr,
	}, nil
}
