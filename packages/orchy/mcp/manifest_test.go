package mcp_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gz-ia/packages/orchy/mcp"
	"gz-ia/packages/orchy/tools"
)

type dummyPluginDesc struct {
	name    string
	version string
}

func (d dummyPluginDesc) Name() string    { return d.name }
func (d dummyPluginDesc) Version() string { return d.version }

func TestManifestGenerator_ToStandardJsonSchema(t *testing.T) {
	reg := tools.NewToolRegistry()
	gen := mcp.NewManifestGenerator(reg)

	schema := tools.ToolSchema{
		Kind:        "object",
		Description: "A test schema",
		Required:    []string{"query", "limit"},
		Properties: map[string]any{
			"query": map[string]any{
				"kind":        "string",
				"description": "Search query",
			},
			"limit": map[string]any{
				"kind": "integer",
			},
			"tags": map[string]any{
				"kind": "array",
				"items": map[string]any{
					"kind": "string",
				},
			},
			"nested": map[string]any{
				"kind": "object",
				"properties": map[string]any{
					"flag": map[string]any{
						"kind": "boolean",
					},
				},
			},
		},
	}

	result := gen.ToStandardJsonSchema(schema)

	if result["type"] != "object" {
		t.Fatalf("expected root type 'object', got %v", result["type"])
	}
	if result["description"] != "A test schema" {
		t.Fatalf("expected description 'A test schema', got %v", result["description"])
	}

	props, ok := result["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map")
	}

	queryProp, ok := props["query"].(map[string]any)
	if !ok || queryProp["type"] != "string" {
		t.Fatalf("expected query.type == 'string', got %v", queryProp)
	}

	limitProp, ok := props["limit"].(map[string]any)
	if !ok || limitProp["type"] != "integer" {
		t.Fatalf("expected limit.type == 'integer', got %v", limitProp)
	}

	tagsProp, ok := props["tags"].(map[string]any)
	if !ok || tagsProp["type"] != "array" {
		t.Fatalf("expected tags.type == 'array', got %v", tagsProp)
	}
	itemsProp, ok := tagsProp["items"].(map[string]any)
	if !ok || itemsProp["type"] != "string" {
		t.Fatalf("expected tags.items.type == 'string', got %v", itemsProp)
	}

	nestedProp, ok := props["nested"].(map[string]any)
	if !ok || nestedProp["type"] != "object" {
		t.Fatalf("expected nested.type == 'object', got %v", nestedProp)
	}
	nestedProps, ok := nestedProp["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested.properties map")
	}
	flagProp, ok := nestedProps["flag"].(map[string]any)
	if !ok || flagProp["type"] != "boolean" {
		t.Fatalf("expected flag.type == 'boolean', got %v", flagProp)
	}
}

func TestManifestGenerator_ExcludesDeadTools(t *testing.T) {
	reg := tools.NewToolRegistry()

	healthyTool := &tools.FuncTool{
		ToolName:        "healthy_tool",
		ToolDescription: "Healthy tool description",
		ToolSchema: tools.ToolSchema{
			Kind: "object",
		},
		Handler: func(ctx context.Context, input any) (any, error) {
			return "all good", nil
		},
	}

	deadTool := &tools.FuncTool{
		ToolName:        "dead_tool",
		ToolDescription: "Dead tool description",
		ToolSchema: tools.ToolSchema{
			Kind: "object",
		},
		Handler: func(ctx context.Context, input any) (any, error) {
			return nil, errors.New("always fail")
		},
	}

	_, _ = reg.Register(healthyTool)
	pDead, _ := reg.Register(deadTool, tools.ToolProxyOptions{MaxConsecutiveFailures: 2})

	// Trip the deadTool
	_, _ = pDead.Execute(context.Background(), nil)
	_, _ = pDead.Execute(context.Background(), nil)
	if pDead.Status() != tools.ToolStatusDead {
		t.Fatalf("expected tool to be DEAD")
	}

	gen := mcp.NewManifestGenerator(reg)
	mcpTools := gen.GenerateMcpTools()

	if len(mcpTools) != 1 {
		t.Fatalf("expected exactly 1 healthy tool exposed, got %d", len(mcpTools))
	}
	if mcpTools[0].Name != "healthy_tool" {
		t.Fatalf("expected 'healthy_tool', got %q", mcpTools[0].Name)
	}

	// Catalog should still contain all tools with their respective status
	catalog := gen.GenerateCatalog()
	if len(catalog) != 2 {
		t.Fatalf("expected 2 tools in catalog, got %d", len(catalog))
	}
	foundDead := false
	for _, item := range catalog {
		if item.Name == "dead_tool" && item.Status == string(tools.ToolStatusDead) {
			foundDead = true
		}
	}
	if !foundDead {
		t.Fatalf("expected dead_tool with DEAD status in catalog")
	}
}

func TestManifestGenerator_GenerateAiContextMarkdown(t *testing.T) {
	reg := tools.NewToolRegistry()
	_, _ = reg.Register(&tools.FuncTool{
		ToolName:        "sample_tool",
		ToolDescription: "A sample tool for testing",
		ToolSchema: tools.ToolSchema{
			Kind: "object",
			Properties: map[string]any{
				"file": map[string]any{"kind": "string"},
			},
		},
		Handler: func(ctx context.Context, input any) (any, error) { return nil, nil },
	})

	pluginList := []mcp.PluginDescriptor{
		dummyPluginDesc{name: "GitPlugin", version: "1.0.0"},
	}

	gen := mcp.NewManifestGenerator(reg, func() []mcp.PluginDescriptor {
		return pluginList
	})

	md := gen.GenerateAiContextMarkdown("Test Orchestrator")

	if !strings.Contains(md, "# Test Orchestrator - AI Context") {
		t.Fatalf("expected title in markdown, got: %s", md)
	}
	if !strings.Contains(md, "- **GitPlugin** (v1.0.0)") {
		t.Fatalf("expected plugin in markdown, got: %s", md)
	}
	if !strings.Contains(md, "### `sample_tool`") {
		t.Fatalf("expected tool header in markdown, got: %s", md)
	}
	if !strings.Contains(md, "- **Estado**: `HEALTHY`") {
		t.Fatalf("expected status in markdown, got: %s", md)
	}
}
