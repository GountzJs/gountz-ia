package worktree_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gz-ia/internal/features/session"
	"gz-ia/internal/features/workspace"
	"gz-ia/packages/orchy"
	"gz-ia/packages/orchy/batteries/worktree"
	"gz-ia/packages/orchy/mcp"
)

// initGitRepo initializes a real git repository with an initial commit in dir.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@gountz.com")
	runCmd(t, dir, "git", "config", "user.name", "Gountz Tester")
	runCmd(t, dir, "git", "config", "commit.gpgsign", "false")

	initFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(initFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}
	runCmd(t, dir, "git", "add", "README.md")
	runCmd(t, dir, "git", "commit", "-m", "Initial commit")
}

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v, output: %s", name, args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestWorktreePlugin_MetadataAndOptions(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	ws := workspace.NewProvider()
	svc := session.NewService(tmpDir, session.WithWorkspace(ws))

	p := worktree.NewWorktreePlugin(
		tmpDir,
		worktree.WithSessionService(svc),
		worktree.WithWorkspaceProvider(ws),
		worktree.WithBaseDir(tmpDir),
		worktree.WithAllowAgentGet(true),
	)

	if p.Name() != "batteries.worktree" {
		t.Errorf("expected plugin name 'batteries.worktree', got %q", p.Name())
	}
	if p.Version() != "0.0.1" {
		t.Errorf("expected plugin version '0.0.1', got %q", p.Version())
	}
	if p.GetReadTool() == nil {
		t.Error("expected ReadTool to be non-nil")
	}
	if p.GetGetTool() == nil {
		t.Error("expected GetTool to be non-nil")
	}
	if p.GetSessionService() != svc {
		t.Error("expected custom session service to be retained")
	}
	if err := p.OnShutdown(nil); err != nil {
		t.Errorf("OnShutdown failed: %v", err)
	}
	if err := p.OnBoot(nil); err == nil {
		t.Error("OnBoot(nil) should fail with nil context")
	}
}

func TestWorktreePlugin_ToolsExecution_RealGit(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	initGitRepo(t, baseDir)

	ws := workspace.NewProvider()
	svc := session.NewService(baseDir, session.WithWorkspace(ws))

	// Prepare an isolated session worktree
	sessID := "test_sess_01"
	wtPrep, err := ws.Prepare(ctx, sessID, baseDir)
	if err != nil {
		t.Fatalf("failed to prepare worktree: %v", err)
	}
	if !wtPrep.IsIsolated {
		t.Fatal("expected worktree to be isolated")
	}

	// Register session in store
	store := session.DefaultFileStore(baseDir)
	now := time.Now()
	err = store.Save(&session.SessionRecord{
		ID:          sessID,
		Status:      session.StatusCompleted,
		WorkingDir:  baseDir,
		IsIsolated:  true,
		WorktreeDir: wtPrep.WorktreeDir,
		BranchName:  wtPrep.BranchName,
		StartedAt:   now,
	})
	if err != nil {
		t.Fatalf("failed to save session record: %v", err)
	}

	// Write modifications inside the worktree
	featureFile := filepath.Join(wtPrep.WorktreeDir, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Orchy battery worktree feature\n"), 0644); err != nil {
		t.Fatalf("failed to write to worktree file: %v", err)
	}
	runCmd(t, wtPrep.WorktreeDir, "git", "add", "feature.txt")
	runCmd(t, wtPrep.WorktreeDir, "git", "commit", "-m", "Add feature in worktree")

	// Set up Orchy Kernel and Plugin
	kernel := orchy.NewKernel()
	plugin := worktree.NewWorktreePlugin(baseDir, worktree.WithSessionService(svc))
	if err := kernel.Use(plugin); err != nil {
		t.Fatalf("failed to register plugin: %v", err)
	}

	if err := kernel.Boot(ctx); err != nil {
		t.Fatalf("failed to boot kernel: %v", err)
	}
	defer func() { _ = kernel.Shutdown(ctx) }()

	kCtx := kernel.GetContext()

	// 1. Execute worktree_read (diff full)
	readInput := map[string]any{
		"session_id": sessID,
		"stat_only":  false,
	}
	readResRaw, err := kCtx.ExecuteTool(ctx, "worktree_read", readInput)
	if err != nil {
		t.Fatalf("worktree_read failed: %v", err)
	}

	readRes, ok := readResRaw.(worktree.ReadToolOutput)
	if !ok {
		// In case it's returned by value or pointer
		t.Fatalf("unexpected type for worktree_read output: %T", readResRaw)
	}
	if readRes.SessionID != sessID {
		t.Errorf("expected session_id %q, got %q", sessID, readRes.SessionID)
	}
	if !strings.Contains(readRes.Diff, "feature.txt") || !strings.Contains(readRes.Diff, "Orchy battery worktree feature") {
		t.Errorf("worktree_read diff missing expected content:\n%s", readRes.Diff)
	}

	// 2. Execute worktree_read with stat_only
	statInput := map[string]any{
		"session_id": sessID,
		"stat_only":  true,
	}
	statResRaw, err := kCtx.ExecuteTool(ctx, "worktree_read", statInput)
	if err != nil {
		t.Fatalf("worktree_read stat_only failed: %v", err)
	}
	statRes := statResRaw.(worktree.ReadToolOutput)
	if !statRes.StatOnly {
		t.Errorf("expected StatOnly to be true")
	}
	if !strings.Contains(statRes.Diff, "feature.txt") {
		t.Errorf("worktree_read stat missing file mention:\n%s", statRes.Diff)
	}

	// 3. Verify worktree_get is NOT registered by default for security / human-in-the-loop control
	getInput := map[string]any{
		"session_id": sessID,
		"squash":     false,
		"no_commit":  false,
	}
	if _, err := kCtx.ExecuteTool(ctx, "worktree_get", getInput); err == nil {
		t.Fatal("expected worktree_get to NOT be registered in default mode")
	}

	// 4. Register worktree_get explicitly using WithAllowAgentGet(true)
	optKernel := orchy.NewKernel()
	optPlugin := worktree.NewWorktreePlugin(baseDir, worktree.WithSessionService(svc), worktree.WithAllowAgentGet(true))
	if err := optKernel.Use(optPlugin); err != nil {
		t.Fatalf("failed to register optPlugin: %v", err)
	}
	if err := optKernel.Boot(ctx); err != nil {
		t.Fatalf("failed to boot optKernel: %v", err)
	}
	defer func() { _ = optKernel.Shutdown(ctx) }()

	getResRaw, err := optKernel.GetContext().ExecuteTool(ctx, "worktree_get", getInput)
	if err != nil {
		t.Fatalf("worktree_get failed: %v", err)
	}
	getRes := getResRaw.(worktree.GetToolOutput)
	if !getRes.Success {
		t.Errorf("expected success true, got false. Message: %s", getRes.Message)
	}
	if len(getRes.Files) == 0 || getRes.Files[0] != "feature.txt" {
		t.Errorf("expected files integrated ['feature.txt'], got: %v", getRes.Files)
	}

	// Verify file is now merged into base repo
	mergedFilePath := filepath.Join(baseDir, "feature.txt")
	content, err := os.ReadFile(mergedFilePath)
	if err != nil {
		t.Fatalf("merged file %s not found in base dir: %v", mergedFilePath, err)
	}
	if !strings.Contains(string(content), "Orchy battery worktree feature") {
		t.Errorf("file content mismatch: %s", string(content))
	}
}

func TestWorktreePlugin_McpServerIntegration(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()
	initGitRepo(t, baseDir)

	ws := workspace.NewProvider()
	svc := session.NewService(baseDir, session.WithWorkspace(ws))

	// Prepare session worktree
	sessID := "mcp_sess_01"
	wtPrep, err := ws.Prepare(ctx, sessID, baseDir)
	if err != nil {
		t.Fatalf("failed to prepare worktree: %v", err)
	}

	store := session.DefaultFileStore(baseDir)
	_ = store.Save(&session.SessionRecord{
		ID:          sessID,
		Status:      session.StatusCompleted,
		WorkingDir:  baseDir,
		IsIsolated:  true,
		WorktreeDir: wtPrep.WorktreeDir,
		BranchName:  wtPrep.BranchName,
		StartedAt:   time.Now(),
	})

	// Add file in worktree
	testFile := filepath.Join(wtPrep.WorktreeDir, "mcp.txt")
	_ = os.WriteFile(testFile, []byte("mcp integration test\n"), 0644)
	runCmd(t, wtPrep.WorktreeDir, "git", "add", "mcp.txt")
	runCmd(t, wtPrep.WorktreeDir, "git", "commit", "-m", "commit for mcp")

	kernel := orchy.NewKernel()
	plugin := worktree.NewWorktreePlugin(baseDir, worktree.WithSessionService(svc))
	_ = kernel.Use(plugin)
	_ = kernel.Boot(ctx)
	defer func() { _ = kernel.Shutdown(ctx) }()

	// Create MCP Server wrapping kernel
	srv := orchy.NewMcpServer(kernel, nil, nil)

	// Test tools/list
	listReq := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	listRespBytes, err := srv.HandleRequest(ctx, []byte(listReq))
	if err != nil {
		t.Fatalf("MCP tools/list failed: %v", err)
	}

	var listResp mcp.JSONRPCResponse
	if err := json.Unmarshal(listRespBytes, &listResp); err != nil {
		t.Fatalf("failed to unmarshal listResp: %v", err)
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %+v", listResp.Error)
	}

	resMap := listResp.Result.(map[string]any)
	toolsList := resMap["tools"].([]any)

	foundRead := false
	foundGet := false
	for _, toolItem := range toolsList {
		tObj := toolItem.(map[string]any)
		if tObj["name"] == "worktree_read" {
			foundRead = true
		}
		if tObj["name"] == "worktree_get" {
			foundGet = true
		}
	}
	if !foundRead {
		t.Fatalf("expected worktree_read in tools/list, foundRead=%v", foundRead)
	}
	if foundGet {
		t.Fatalf("worktree_get should NOT be present in tools/list by default for agent safety")
	}

	// Test tools/call worktree_read succeeds
	readCall := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"worktree_read","arguments":{"session_id":"mcp_sess_01"}}}`
	readRespBytes, err := srv.HandleRequest(ctx, []byte(readCall))
	if err != nil {
		t.Fatalf("MCP call worktree_read failed: %v", err)
	}

	var readResp mcp.JSONRPCResponse
	if err := json.Unmarshal(readRespBytes, &readResp); err != nil {
		t.Fatalf("failed to unmarshal readResp: %v", err)
	}
	if readResp.Error != nil {
		t.Fatalf("tools/call worktree_read error: %+v", readResp.Error)
	}

	callRes := readResp.Result.(map[string]any)
	contentList := callRes["content"].([]any)
	firstContent := contentList[0].(map[string]any)
	textVal := firstContent["text"].(string)
	if !strings.Contains(textVal, "mcp.txt") {
		t.Errorf("worktree_read output text missing 'mcp.txt': %s", textVal)
	}

	// Test tools/call worktree_get fails in default mode (tool not registered)
	getCall := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"worktree_get","arguments":{"session_id":"mcp_sess_01"}}}`
	getRespBytes, err := srv.HandleRequest(ctx, []byte(getCall))
	if err != nil {
		t.Fatalf("MCP call worktree_get request failed: %v", err)
	}
	var getRespDef mcp.JSONRPCResponse
	if err := json.Unmarshal(getRespBytes, &getRespDef); err != nil {
		t.Fatalf("failed to unmarshal getRespDef: %v", err)
	}
	callGetResDef := getRespDef.Result.(map[string]any)
	if callGetResDef["isError"] != true {
		t.Errorf("expected isError=true when calling unregistered worktree_get, got %v", callGetResDef["isError"])
	}

	// Now test MCP server with WithAllowAgentGet(true) opt-in
	optKernel := orchy.NewKernel()
	optPlugin := worktree.NewWorktreePlugin(baseDir, worktree.WithSessionService(svc), worktree.WithAllowAgentGet(true))
	_ = optKernel.Use(optPlugin)
	_ = optKernel.Boot(ctx)
	defer func() { _ = optKernel.Shutdown(ctx) }()

	optSrv := orchy.NewMcpServer(optKernel, nil, nil)
	optGetRespBytes, err := optSrv.HandleRequest(ctx, []byte(getCall))
	if err != nil {
		t.Fatalf("MCP call worktree_get opt-in failed: %v", err)
	}
	var optGetResp mcp.JSONRPCResponse
	if err := json.Unmarshal(optGetRespBytes, &optGetResp); err != nil {
		t.Fatalf("failed to unmarshal optGetResp: %v", err)
	}
	if optGetResp.Error != nil {
		t.Fatalf("opt-in tools/call worktree_get error: %+v", optGetResp.Error)
	}

	callGetRes := optGetResp.Result.(map[string]any)
	getContentList := callGetRes["content"].([]any)
	getTextVal := getContentList[0].(map[string]any)["text"].(string)
	if !strings.Contains(getTextVal, "mcp.txt") && !strings.Contains(getTextVal, "true") {
		t.Errorf("worktree_get output text unexpected: %s", getTextVal)
	}

	// Confirm file merged into base
	if _, err := os.Stat(filepath.Join(baseDir, "mcp.txt")); err != nil {
		t.Errorf("file mcp.txt not integrated into base directory: %v", err)
	}
}

func TestWorktreePlugin_ValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	baseDir := t.TempDir()

	ws := workspace.NewProvider()
	svc := session.NewService(baseDir, session.WithWorkspace(ws))

	readTool := worktree.NewWorktreeReadTool(svc)
	getTool := worktree.NewWorktreeGetTool(svc)

	// 1. Missing session_id
	if _, err := readTool.Execute(ctx, map[string]any{}); err == nil {
		t.Error("worktree_read should fail when session_id is missing")
	}
	if _, err := getTool.Execute(ctx, map[string]any{}); err == nil {
		t.Error("worktree_get should fail when session_id is missing")
	}

	// 2. Non-existent session
	if _, err := readTool.Execute(ctx, map[string]any{"session_id": "non_existent"}); err == nil {
		t.Error("worktree_read should fail for non-existent session")
	}
	if _, err := getTool.Execute(ctx, map[string]any{"session_id": "non_existent"}); err == nil {
		t.Error("worktree_get should fail for non-existent session")
	}

	// 3. Direct (non-isolated) session
	store := session.DefaultFileStore(baseDir)
	_ = store.Save(&session.SessionRecord{
		ID:         "direct_sess",
		Status:     session.StatusCompleted,
		WorkingDir: baseDir,
		IsIsolated: false,
	})

	readOutRaw, err := readTool.Execute(ctx, map[string]any{"session_id": "direct_sess"})
	if err != nil {
		t.Fatalf("unexpected error for direct session read: %v", err)
	}
	readOut := readOutRaw.(worktree.ReadToolOutput)
	if !strings.Contains(readOut.Diff, "modo directo") {
		t.Errorf("expected direct mode message in diff, got: %s", readOut.Diff)
	}

	// worktree_get on direct session must fail
	if _, err := getTool.Execute(ctx, map[string]any{"session_id": "direct_sess"}); err == nil {
		t.Error("worktree_get should fail on non-isolated session")
	}

	// 4. Nil session service
	nilReadTool := worktree.NewWorktreeReadTool(nil)
	if _, err := nilReadTool.Execute(ctx, map[string]any{"session_id": "any"}); err == nil {
		t.Error("expected error when sessionService is nil")
	}
	nilGetTool := worktree.NewWorktreeGetTool(nil)
	if _, err := nilGetTool.Execute(ctx, map[string]any{"session_id": "any"}); err == nil {
		t.Error("expected error when sessionService is nil")
	}

	// 5. Schema verification
	readSchema := readTool.Schema()
	if len(readSchema.Required) != 1 || readSchema.Required[0] != "session_id" {
		t.Errorf("unexpected schema required for read: %v", readSchema.Required)
	}
	getSchema := getTool.Schema()
	if len(getSchema.Required) != 1 || getSchema.Required[0] != "session_id" {
		t.Errorf("unexpected schema required for get: %v", getSchema.Required)
	}
}
