package orchy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"gz-ia/packages/orchy"
)

func TestOrchy_Facade(t *testing.T) {
	kernel := orchy.NewKernel()
	ctx := context.Background()

	// 1. NewTool
	customTool := orchy.NewTool(
		"greet",
		"Greets a user by name",
		orchy.ToolSchema{
			Kind: "object",
			Properties: map[string]any{
				"name": map[string]any{"kind": "string"},
			},
			Required: []string{"name"},
		},
		func(ctx context.Context, input any) (any, error) {
			m, _ := input.(map[string]any)
			name := "stranger"
			if m != nil && m["name"] != nil {
				name = m["name"].(string)
			}
			return "Hello, " + name, nil
		},
	)

	_, err := kernel.RegisterTool(customTool)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	// 2. NewPlugin
	var booted bool
	var shutdown bool
	plugin := orchy.NewPlugin(
		"TelemetryPlugin",
		"1.1.0",
		func(ctx *orchy.KernelContext) error {
			booted = true
			return ctx.RegisterService("telemetry_active", true)
		},
		func(ctx *orchy.KernelContext) error {
			shutdown = true
			return nil
		},
	)

	if err := kernel.Use(plugin); err != nil {
		t.Fatalf("failed to use plugin: %v", err)
	}

	// 3. Lifecycle
	if err := kernel.Boot(ctx); err != nil {
		t.Fatalf("kernel boot failed: %v", err)
	}
	if !booted {
		t.Fatalf("plugin was not booted")
	}

	svcVal, err := kernel.GetContext().GetService("telemetry_active")
	if err != nil || svcVal != true {
		t.Fatalf("expected telemetry_active == true, got %v", svcVal)
	}

	// 4. MCP Server integration
	inBuf := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"greet","arguments":{"name":"Gountz"}}}` + "\n")
	outBuf := &bytes.Buffer{}

	mcpSrv := orchy.NewMcpServer(kernel, inBuf, outBuf)
	if err := mcpSrv.Serve(ctx); err != nil {
		t.Fatalf("mcp server failed: %v", err)
	}

	var rpcResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(outBuf.Bytes(), &rpcResp); err != nil {
		t.Fatalf("failed to unmarshal mcp response: %v", err)
	}
	if rpcResp.Result.IsError || rpcResp.Result.Content[0].Text != "Hello, Gountz" {
		t.Fatalf("unexpected MCP call response: %+v", rpcResp)
	}

	// 5. Shutdown
	if err := kernel.Shutdown(ctx); err != nil {
		t.Fatalf("kernel shutdown failed: %v", err)
	}
	if !shutdown {
		t.Fatalf("plugin was not shutdown")
	}
}

func TestOrchy_Constructors(t *testing.T) {
	sc := orchy.NewServiceContainer()
	if sc == nil {
		t.Fatalf("expected non-nil ServiceContainer")
	}

	eb := orchy.NewEventBus()
	if eb == nil {
		t.Fatalf("expected non-nil EventBus")
	}

	tr := orchy.NewToolRegistry()
	if tr == nil {
		t.Fatalf("expected non-nil ToolRegistry")
	}

	pr := orchy.NewPluginRegistry()
	if pr == nil {
		t.Fatalf("expected non-nil PluginRegistry")
	}

	mg := orchy.NewManifestGenerator(tr)
	if mg == nil {
		t.Fatalf("expected non-nil ManifestGenerator")
	}
}
