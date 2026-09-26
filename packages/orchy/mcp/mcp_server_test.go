package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"gz-ia/packages/orchy/mcp"
	"gz-ia/packages/orchy/tools"
)

// mockExecutor implements mcp.ToolExecutor.
type mockExecutor struct {
	execFn func(ctx context.Context, name string, input any) (any, error)
}

func (m *mockExecutor) ExecuteTool(ctx context.Context, name string, input any) (any, error) {
	if m.execFn != nil {
		return m.execFn(ctx, name, input)
	}
	return nil, nil
}

func TestMcpServer_HandleRequest_Initialize(t *testing.T) {
	reg := tools.NewToolRegistry()
	gen := mcp.NewManifestGenerator(reg)
	srv := mcp.NewServer(gen, nil, nil, nil)
	srv.SetServerInfo("custom-server", "2.0.0")

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	respBytes, err := srv.HandleRequest(context.Background(), []byte(req))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("expected no error, got: %+v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map")
	}

	serverInfo, ok := resMap["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "custom-server" || serverInfo["version"] != "2.0.0" {
		t.Fatalf("unexpected serverInfo: %v", serverInfo)
	}
}

func TestMcpServer_HandleRequest_ToolsListAndCall(t *testing.T) {
	reg := tools.NewToolRegistry()
	calcTool := &tools.FuncTool{
		ToolName:        "calculate",
		ToolDescription: "Performs math calculations",
		ToolSchema: tools.ToolSchema{
			Kind: "object",
			Properties: map[string]any{
				"expression": map[string]any{"kind": "string"},
			},
		},
		Handler: func(ctx context.Context, input any) (any, error) {
			return "result is 42", nil
		},
	}
	_, _ = reg.Register(calcTool)

	gen := mcp.NewManifestGenerator(reg)
	exec := &mockExecutor{
		execFn: func(ctx context.Context, name string, input any) (any, error) {
			if name == "calculate" {
				return "result is 42", nil
			}
			return nil, errors.New("tool not found")
		},
	}

	srv := mcp.NewServer(gen, exec, nil, nil)

	// 1. tools/list
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	listRespBytes, err := srv.HandleRequest(context.Background(), []byte(listReq))
	if err != nil {
		t.Fatalf("unexpected error on tools/list: %v", err)
	}

	var listResp mcp.JSONRPCResponse
	if err := json.Unmarshal(listRespBytes, &listResp); err != nil {
		t.Fatalf("failed to unmarshal tools/list response: %v", err)
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list returned error: %+v", listResp.Error)
	}

	resMap := listResp.Result.(map[string]any)
	toolsList := resMap["tools"].([]any)
	if len(toolsList) != 1 {
		t.Fatalf("expected 1 tool in list, got %d", len(toolsList))
	}
	toolItem := toolsList[0].(map[string]any)
	if toolItem["name"] != "calculate" {
		t.Fatalf("expected tool name 'calculate', got %v", toolItem["name"])
	}

	// 2. tools/call success
	callReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"calculate","arguments":{"expression":"6*7"}}}`
	callRespBytes, err := srv.HandleRequest(context.Background(), []byte(callReq))
	if err != nil {
		t.Fatalf("unexpected error on tools/call: %v", err)
	}

	var callResp mcp.JSONRPCResponse
	if err := json.Unmarshal(callRespBytes, &callResp); err != nil {
		t.Fatalf("failed to unmarshal tools/call response: %v", err)
	}

	callResMap := callResp.Result.(map[string]any)
	if isErr, ok := callResMap["isError"].(bool); ok && isErr {
		t.Fatalf("expected isError == false")
	}
	contents := callResMap["content"].([]any)
	firstContent := contents[0].(map[string]any)
	if firstContent["text"] != "result is 42" {
		t.Fatalf("expected text 'result is 42', got %v", firstContent["text"])
	}

	// 3. tools/call failure
	failReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"missing_tool"}}`
	failRespBytes, err := srv.HandleRequest(context.Background(), []byte(failReq))
	if err != nil {
		t.Fatalf("unexpected error on failing tools/call: %v", err)
	}

	var failResp mcp.JSONRPCResponse
	if err := json.Unmarshal(failRespBytes, &failResp); err != nil {
		t.Fatalf("failed to unmarshal fail response: %v", err)
	}

	failResMap := failResp.Result.(map[string]any)
	if isErr, ok := failResMap["isError"].(bool); !ok || !isErr {
		t.Fatalf("expected isError == true for failing tool")
	}
}

func TestMcpServer_MethodNotFoundAndParseError(t *testing.T) {
	srv := mcp.NewServer(nil, nil, nil, nil)

	// Unknown method
	unknownReq := `{"jsonrpc":"2.0","id":10,"method":"unknown/method"}`
	respBytes, _ := srv.HandleRequest(context.Background(), []byte(unknownReq))
	var resp mcp.JSONRPCResponse
	_ = json.Unmarshal(respBytes, &resp)
	if resp.Error == nil || resp.Error.Code != mcp.CodeMethodNotFound {
		t.Fatalf("expected CodeMethodNotFound (-32601), got %+v", resp.Error)
	}

	// Bad JSON
	badReq := `{"jsonrpc": broken`
	respBytes, _ = srv.HandleRequest(context.Background(), []byte(badReq))
	_ = json.Unmarshal(respBytes, &resp)
	if resp.Error == nil || resp.Error.Code != mcp.CodeParseError {
		t.Fatalf("expected CodeParseError (-32700), got %+v", resp.Error)
	}

	// Invalid Request (no 2.0)
	invalidVersion := `{"jsonrpc":"1.0","id":11,"method":"ping"}`
	respBytes, _ = srv.HandleRequest(context.Background(), []byte(invalidVersion))
	_ = json.Unmarshal(respBytes, &resp)
	if resp.Error == nil || resp.Error.Code != mcp.CodeInvalidRequest {
		t.Fatalf("expected CodeInvalidRequest (-32600), got %+v", resp.Error)
	}
}

func TestMcpServer_Serve_StdioLoop(t *testing.T) {
	inputLines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize"}`,
	}
	input := strings.Join(inputLines, "\n") + "\n"

	inBuf := bytes.NewBufferString(input)
	outBuf := &bytes.Buffer{}

	srv := mcp.NewServer(nil, nil, inBuf, outBuf)
	err := srv.Serve(context.Background())
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	output := outBuf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	// Notice notification should not produce output line, so we expect 2 responses
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d. Output: %s", len(lines), output)
	}

	var resp1, resp2 mcp.JSONRPCResponse
	_ = json.Unmarshal([]byte(lines[0]), &resp1)
	_ = json.Unmarshal([]byte(lines[1]), &resp2)

	if resp1.ID != float64(1) || resp2.ID != float64(2) {
		t.Fatalf("unexpected response IDs: resp1=%v, resp2=%v", resp1.ID, resp2.ID)
	}
}

func TestMcpServer_CircuitBreaker_Isolation(t *testing.T) {
	// Test that repeated failures trip the circuit breaker through the registry,
	// protecting the server and cleanly reporting errors without crashing.
	reg := tools.NewToolRegistry()
	var failCounter int32

	buggyTool := &tools.FuncTool{
		ToolName:        "buggy",
		ToolDescription: "A buggy tool that crashes",
		Handler: func(ctx context.Context, input any) (any, error) {
			atomic.AddInt32(&failCounter, 1)
			return nil, errors.New("database connection refused")
		},
	}

	_, _ = reg.Register(buggyTool, tools.ToolProxyOptions{MaxConsecutiveFailures: 3})
	gen := mcp.NewManifestGenerator(reg)

	// Executor delegates to ToolRegistry
	exec := &mockExecutor{
		execFn: func(ctx context.Context, name string, input any) (any, error) {
			proxy, ok := reg.Get(name)
			if !ok {
				return nil, errors.New("tool not found")
			}
			return proxy.Execute(ctx, input)
		},
	}

	srv := mcp.NewServer(gen, exec, nil, nil)

	// 1. Initial tools/list contains "buggy"
	toolsBefore := gen.GenerateMcpTools()
	if len(toolsBefore) != 1 {
		t.Fatalf("expected buggy tool in list initially")
	}

	// 2. Call tool 3 times to trip breaker
	for i := 1; i <= 3; i++ {
		callReq := `{"jsonrpc":"2.0","id":` + string(rune('0'+i)) + `,"method":"tools/call","params":{"name":"buggy"}}`
		respBytes, err := srv.HandleRequest(context.Background(), []byte(callReq))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var resp mcp.JSONRPCResponse
		_ = json.Unmarshal(respBytes, &resp)
		callRes := resp.Result.(map[string]any)
		if !callRes["isError"].(bool) {
			t.Fatalf("expected isError to be true on failure %d", i)
		}
	}

	// Tool should now be DEAD
	p, _ := reg.Get("buggy")
	if p.Status() != tools.ToolStatusDead {
		t.Fatalf("expected tool status DEAD, got %v", p.Status())
	}

	// 3. Check tools/list: DEAD tool MUST be omitted!
	toolsAfter := gen.GenerateMcpTools()
	if len(toolsAfter) != 0 {
		t.Fatalf("expected DEAD tool to be excluded from tools/list, got %d tools", len(toolsAfter))
	}

	// 4. Calling DEAD tool trips circuit breaker immediately without calling tool handler
	callReqDead := `{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"buggy"}}`
	respBytes, _ := srv.HandleRequest(context.Background(), []byte(callReqDead))
	var resp mcp.JSONRPCResponse
	_ = json.Unmarshal(respBytes, &resp)
	callRes := resp.Result.(map[string]any)
	if !callRes["isError"].(bool) {
		t.Fatalf("expected isError true for DEAD tool")
	}
	content := callRes["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(content, "circuit breaker tripped") {
		t.Fatalf("expected 'circuit breaker tripped' in error content, got: %s", content)
	}

	// Handler was called exactly 3 times, not 4
	if atomic.LoadInt32(&failCounter) != 3 {
		t.Fatalf("expected handler called 3 times, got %d", atomic.LoadInt32(&failCounter))
	}
}
