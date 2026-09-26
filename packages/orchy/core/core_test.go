package core_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"gz-ia/packages/orchy/core"
	"gz-ia/packages/orchy/tools"
)

// mockPlugin implements core.Plugin for testing.
type mockPlugin struct {
	name       string
	version    string
	bootErr    error
	bootFn     func(ctx *core.KernelContext) error
	shutdownFn func(ctx *core.KernelContext) error
}

func (m *mockPlugin) Name() string    { return m.name }
func (m *mockPlugin) Version() string { return m.version }
func (m *mockPlugin) OnBoot(ctx *core.KernelContext) error {
	if m.bootErr != nil {
		return m.bootErr
	}
	if m.bootFn != nil {
		return m.bootFn(ctx)
	}
	return nil
}
func (m *mockPlugin) OnShutdown(ctx *core.KernelContext) error {
	if m.shutdownFn != nil {
		return m.shutdownFn(ctx)
	}
	return nil
}

func TestServiceContainer(t *testing.T) {
	c := core.NewServiceContainer()

	// Register and Get
	err := c.Register("db", "postgres-connection")
	if err != nil {
		t.Fatalf("unexpected error registering service: %v", err)
	}

	// Duplicate registration error
	err = c.Register("db", "another-connection")
	if err == nil {
		t.Fatalf("expected error on duplicate service registration")
	}

	// Empty name registration error
	err = c.Register("", "invalid")
	if err == nil {
		t.Fatalf("expected error on empty service name")
	}

	// Has
	if !c.Has("db") {
		t.Fatalf("expected Has('db') to be true")
	}
	if c.Has("non-existent") {
		t.Fatalf("expected Has('non-existent') to be false")
	}

	// Get existing
	val, err := c.Get("db")
	if err != nil || val != "postgres-connection" {
		t.Fatalf("expected 'postgres-connection', got val=%v, err=%v", val, err)
	}

	// Get non-existing
	_, err = c.Get("missing")
	if err == nil {
		t.Fatalf("expected error on missing service")
	}

	// TryGet
	v, ok := c.TryGet("db")
	if !ok || v != "postgres-connection" {
		t.Fatalf("expected TryGet to succeed")
	}
	_, ok = c.TryGet("missing")
	if ok {
		t.Fatalf("expected TryGet to return false for missing")
	}

	// List
	list := c.List()
	if len(list) != 1 || list[0] != "db" {
		t.Fatalf("expected List() to contain 'db', got %v", list)
	}

	// Remove
	if !c.Remove("db") {
		t.Fatalf("expected Remove to return true")
	}
	if c.Remove("db") {
		t.Fatalf("expected second Remove to return false")
	}

	// Clear
	_ = c.Register("s1", 1)
	_ = c.Register("s2", 2)
	c.Clear()
	if len(c.List()) != 0 {
		t.Fatalf("expected empty container after Clear()")
	}
}

func TestServiceContainer_Concurrency(t *testing.T) {
	c := core.NewServiceContainer()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("svc_%d", idx)
			_ = c.Register(name, idx)
			_, _ = c.Get(name)
			_ = c.Has(name)
			_ = c.List()
		}(i)
	}
	wg.Wait()

	if len(c.List()) != 50 {
		t.Fatalf("expected 50 services registered concurrently, got %d", len(c.List()))
	}
}

func TestKernel_Lifecycle(t *testing.T) {
	k := core.NewKernel()
	ctx := context.Background()

	if k.State() != core.KernelStateIdle {
		t.Fatalf("expected state IDLE, got %v", k.State())
	}
	if k.IsRunning() {
		t.Fatalf("expected IsRunning to be false")
	}

	var eventsSeen []string
	var mu sync.Mutex
	recordEvent := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		eventsSeen = append(eventsSeen, name)
	}

	k.GetContext().On("kernel:booting", func(ctx context.Context, payload any) error {
		recordEvent("kernel:booting")
		return nil
	})
	k.GetContext().On("kernel:ready", func(ctx context.Context, payload any) error {
		recordEvent("kernel:ready")
		return nil
	})
	k.GetContext().On("kernel:shutting_down", func(ctx context.Context, payload any) error {
		recordEvent("kernel:shutting_down")
		return nil
	})

	var executionOrder []string
	p1 := &mockPlugin{
		name:    "p1",
		version: "1.0.0",
		bootFn: func(ctx *core.KernelContext) error {
			executionOrder = append(executionOrder, "boot:p1")
			return nil
		},
		shutdownFn: func(ctx *core.KernelContext) error {
			executionOrder = append(executionOrder, "shutdown:p1")
			return nil
		},
	}
	p2 := &mockPlugin{
		name:    "p2",
		version: "2.0.0",
		bootFn: func(ctx *core.KernelContext) error {
			executionOrder = append(executionOrder, "boot:p2")
			return nil
		},
		shutdownFn: func(ctx *core.KernelContext) error {
			executionOrder = append(executionOrder, "shutdown:p2")
			return nil
		},
	}

	if err := k.Use(p1); err != nil {
		t.Fatalf("unexpected error adding p1: %v", err)
	}
	if err := k.Use(p2); err != nil {
		t.Fatalf("unexpected error adding p2: %v", err)
	}

	// Duplicate plugin registration error
	if err := k.Use(p1); err == nil {
		t.Fatalf("expected error registering duplicate plugin")
	}

	// Boot Kernel
	err := k.Boot(ctx)
	if err != nil {
		t.Fatalf("unexpected error booting kernel: %v", err)
	}

	if k.State() != core.KernelStateRunning {
		t.Fatalf("expected state RUNNING, got %v", k.State())
	}
	if !k.IsRunning() {
		t.Fatalf("expected IsRunning to be true")
	}

	// Re-booting when already running should be a safe no-op
	if err := k.Boot(ctx); err != nil {
		t.Fatalf("re-booting running kernel returned error: %v", err)
	}

	// Add plugin while kernel is already RUNNING -> should immediately boot
	var p3Booted bool
	p3 := &mockPlugin{
		name:    "p3",
		version: "1.0.0",
		bootFn: func(ctx *core.KernelContext) error {
			p3Booted = true
			executionOrder = append(executionOrder, "boot:p3")
			return nil
		},
		shutdownFn: func(ctx *core.KernelContext) error {
			executionOrder = append(executionOrder, "shutdown:p3")
			return nil
		},
	}
	if err := k.Use(p3); err != nil {
		t.Fatalf("failed to add p3 to running kernel: %v", err)
	}
	if !p3Booted {
		t.Fatalf("expected p3 to boot immediately when added to running kernel")
	}

	// Shutdown Kernel
	err = k.Shutdown(ctx)
	if err != nil {
		t.Fatalf("unexpected error shutting down kernel: %v", err)
	}

	if k.State() != core.KernelStateStopped {
		t.Fatalf("expected state STOPPED, got %v", k.State())
	}

	// Verify boot and shutdown order
	// Boot: p1, p2, p3
	// Shutdown: p3, p2, p1 (reverse order)
	expectedOrder := []string{"boot:p1", "boot:p2", "boot:p3", "shutdown:p3", "shutdown:p2", "shutdown:p1"}
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("expected order %v, got %v", expectedOrder, executionOrder)
	}
	for i, exp := range expectedOrder {
		if executionOrder[i] != exp {
			t.Fatalf("step %d: expected %s, got %s", i, exp, executionOrder[i])
		}
	}

	// Verify events
	if len(eventsSeen) < 3 {
		t.Fatalf("expected at least 3 lifecycle events, got %v", eventsSeen)
	}
}

func TestKernel_Boot_PluginError(t *testing.T) {
	k := core.NewKernel()
	failingPlugin := &mockPlugin{
		name:    "failing",
		bootErr: errors.New("boot failure simulation"),
	}

	_ = k.Use(failingPlugin)

	err := k.Boot(context.Background())
	if err == nil {
		t.Fatalf("expected error when booting failing plugin")
	}
	if k.State() != core.KernelStateStopped {
		t.Fatalf("expected kernel to be STOPPED after boot failure, got %v", k.State())
	}
}

func TestKernelContext_Operations(t *testing.T) {
	k := core.NewKernel()
	ctx := context.Background()
	kCtx := k.GetContext()

	// Services
	_ = kCtx.RegisterService("api_key", "secret123")
	val, err := kCtx.GetService("api_key")
	if err != nil || val != "secret123" {
		t.Fatalf("expected secret123, got %v, err=%v", val, err)
	}

	// Tools registration & execution
	echoTool := &tools.FuncTool{
		ToolName:        "echo",
		ToolDescription: "Echoes input",
		Handler: func(ctx context.Context, input any) (any, error) {
			return input, nil
		},
	}

	_, err = kCtx.RegisterTool(echoTool)
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	out, err := kCtx.ExecuteTool(ctx, "echo", "hello world")
	if err != nil || out != "hello world" {
		t.Fatalf("expected 'hello world', got out=%v, err=%v", out, err)
	}

	// Execute missing tool
	_, err = kCtx.ExecuteTool(ctx, "missing_tool", nil)
	if err == nil {
		t.Fatalf("expected error executing missing tool")
	}

	// Events & RPC through KernelContext
	var received any
	kCtx.On("item:created", func(ctx context.Context, payload any) error {
		received = payload
		return nil
	})
	_ = kCtx.Emit(ctx, "item:created", 42)
	if received != 42 {
		t.Fatalf("expected event payload 42, got %v", received)
	}

	_, _ = kCtx.Respond("multiply", func(ctx context.Context, payload any) (any, error) {
		n := payload.(int)
		return n * 2, nil
	})
	rpcRes, err := kCtx.Request(ctx, "multiply", 21)
	if err != nil || rpcRes.(int) != 42 {
		t.Fatalf("expected RPC response 42, got %v, err=%v", rpcRes, err)
	}

	// Introspector & Services getters
	if k.GetIntrospector() == nil {
		t.Fatalf("expected non-nil Introspector")
	}
	if kCtx.Services() == nil || kCtx.EventBus() == nil || kCtx.Tools() == nil {
		t.Fatalf("expected non-nil accessors on KernelContext")
	}
}
