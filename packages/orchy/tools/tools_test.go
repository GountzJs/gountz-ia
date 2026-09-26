package tools_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gz-ia/packages/orchy/tools"
)

func TestToolProxy_CircuitBreaker_Transitions(t *testing.T) {
	ctx := context.Background()

	var failToggle int32 = 1 // 1 = fail, 0 = succeed
	flakyTool := &tools.FuncTool{
		ToolName:        "flaky",
		ToolDescription: "Flaky test tool",
		ToolSchema: tools.ToolSchema{
			Kind: "object",
		},
		Handler: func(ctx context.Context, input any) (any, error) {
			if atomic.LoadInt32(&failToggle) == 1 {
				return nil, errors.New("simulated failure")
			}
			return "success", nil
		},
	}

	proxy := tools.NewToolProxy(flakyTool, tools.ToolProxyOptions{
		MaxConsecutiveFailures: 4,
		Timeout:                1 * time.Second,
	})

	// Initial status should be HEALTHY
	if proxy.Status() != tools.ToolStatusHealthy {
		t.Fatalf("expected initial status HEALTHY, got %v", proxy.Status())
	}
	if proxy.Name() != "flaky" || proxy.Description() != "Flaky test tool" {
		t.Fatalf("unexpected metadata")
	}

	// 1st failure: remains HEALTHY
	_, err := proxy.Execute(ctx, nil)
	if err == nil {
		t.Fatalf("expected error on failure 1")
	}
	if proxy.Status() != tools.ToolStatusHealthy {
		t.Fatalf("expected status to remain HEALTHY after 1 failure, got %v", proxy.Status())
	}
	if proxy.Metrics().ConsecutiveFailures != 1 {
		t.Fatalf("expected 1 consecutive failure, got %d", proxy.Metrics().ConsecutiveFailures)
	}

	// 2nd failure: transitions to DEGRADED
	_, err = proxy.Execute(ctx, nil)
	if err == nil {
		t.Fatalf("expected error on failure 2")
	}
	if proxy.Status() != tools.ToolStatusDegraded {
		t.Fatalf("expected status DEGRADED after 2 failures, got %v", proxy.Status())
	}

	// 3rd failure: remains DEGRADED
	_, _ = proxy.Execute(ctx, nil)
	if proxy.Status() != tools.ToolStatusDegraded {
		t.Fatalf("expected status DEGRADED after 3 failures, got %v", proxy.Status())
	}

	// 4th failure: reaches MaxConsecutiveFailures (4) -> transitions to DEAD
	_, _ = proxy.Execute(ctx, nil)
	if proxy.Status() != tools.ToolStatusDead {
		t.Fatalf("expected status DEAD after 4 failures, got %v", proxy.Status())
	}
	if proxy.Metrics().ConsecutiveFailures != 4 {
		t.Fatalf("expected 4 consecutive failures, got %d", proxy.Metrics().ConsecutiveFailures)
	}

	// When DEAD, subsequent calls are tripped immediately without executing the tool
	atomic.StoreInt32(&failToggle, 0) // even if tool would now succeed
	_, err = proxy.Execute(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker tripped") {
		t.Fatalf("expected circuit breaker tripped error, got: %v", err)
	}
	if proxy.Metrics().TotalCalls != 4 {
		t.Fatalf("expected calls to remain 4 because DEAD calls don't increment total execution calls, got %d", proxy.Metrics().TotalCalls)
	}

	// Reset circuit breaker
	proxy.ResetCircuitBreaker()
	if proxy.Status() != tools.ToolStatusHealthy {
		t.Fatalf("expected HEALTHY after ResetCircuitBreaker, got %v", proxy.Status())
	}
	if proxy.Metrics().ConsecutiveFailures != 0 {
		t.Fatalf("expected 0 consecutive failures after reset, got %d", proxy.Metrics().ConsecutiveFailures)
	}

	// Now execute again -> succeeds!
	res, err := proxy.Execute(ctx, nil)
	if err != nil || res != "success" {
		t.Fatalf("expected success after reset, got res=%v, err=%v", res, err)
	}
	if proxy.Status() != tools.ToolStatusHealthy {
		t.Fatalf("expected status HEALTHY after success, got %v", proxy.Status())
	}
}

func TestToolProxy_Recovery_From_Degraded(t *testing.T) {
	ctx := context.Background()
	var failToggle int32 = 1

	tool := &tools.FuncTool{
		ToolName: "recovery_test",
		Handler: func(ctx context.Context, input any) (any, error) {
			if atomic.LoadInt32(&failToggle) == 1 {
				return nil, errors.New("err")
			}
			return "recovered", nil
		},
	}

	proxy := tools.NewToolProxy(tool, tools.ToolProxyOptions{
		MaxConsecutiveFailures: 5,
	})

	// Fail twice to enter DEGRADED
	_, _ = proxy.Execute(ctx, nil)
	_, _ = proxy.Execute(ctx, nil)
	if proxy.Status() != tools.ToolStatusDegraded {
		t.Fatalf("expected DEGRADED, got %v", proxy.Status())
	}

	// Switch to success
	atomic.StoreInt32(&failToggle, 0)
	res, err := proxy.Execute(ctx, nil)
	if err != nil || res != "recovered" {
		t.Fatalf("expected success, got %v, err=%v", res, err)
	}

	// Should recover to HEALTHY
	if proxy.Status() != tools.ToolStatusHealthy {
		t.Fatalf("expected status to recover to HEALTHY, got %v", proxy.Status())
	}
	if proxy.Metrics().ConsecutiveFailures != 0 {
		t.Fatalf("expected consecutive failures to reset to 0, got %d", proxy.Metrics().ConsecutiveFailures)
	}
	if proxy.Metrics().SuccessfulCalls != 1 || proxy.Metrics().FailedCalls != 2 {
		t.Fatalf("unexpected call counts: %+v", proxy.Metrics())
	}
}

func TestToolProxy_Timeout(t *testing.T) {
	slowTool := &tools.FuncTool{
		ToolName: "slow",
		Handler: func(ctx context.Context, input any) (any, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return "done", nil
			}
		},
	}

	proxy := tools.NewToolProxy(slowTool, tools.ToolProxyOptions{
		MaxConsecutiveFailures: 3,
		Timeout:                30 * time.Millisecond,
	})

	_, err := proxy.Execute(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	metrics := proxy.Metrics()
	if metrics.FailedCalls != 1 {
		t.Fatalf("expected 1 failed call due to timeout, got %d", metrics.FailedCalls)
	}
}

func TestToolProxy_PanicRecovery(t *testing.T) {
	panickyTool := &tools.FuncTool{
		ToolName: "panic_tool",
		Handler: func(ctx context.Context, input any) (any, error) {
			panic("unexpected tool crash!")
		},
	}

	proxy := tools.NewToolProxy(panickyTool)

	_, err := proxy.Execute(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "panic in tool") {
		t.Fatalf("expected panic recovery error, got: %v", err)
	}

	if proxy.Metrics().FailedCalls != 1 {
		t.Fatalf("expected failed call count = 1, got %d", proxy.Metrics().FailedCalls)
	}
}

func TestToolRegistry(t *testing.T) {
	reg := tools.NewToolRegistry()

	t1 := &tools.FuncTool{
		ToolName:        "t1",
		ToolDescription: "Tool 1",
		Handler:         func(ctx context.Context, input any) (any, error) { return "ok", nil },
	}
	t2 := &tools.FuncTool{
		ToolName:        "t2",
		ToolDescription: "Tool 2",
		Handler:         func(ctx context.Context, input any) (any, error) { return "ok", nil },
	}

	// Register
	p1, err := reg.Register(t1)
	if err != nil || p1 == nil {
		t.Fatalf("failed to register t1: %v", err)
	}

	p2, err := reg.Register(t2, tools.ToolProxyOptions{MaxConsecutiveFailures: 2})
	if err != nil || p2 == nil {
		t.Fatalf("failed to register t2: %v", err)
	}

	// Duplicate error
	_, err = reg.Register(t1)
	if err == nil {
		t.Fatalf("expected error on duplicate tool registration")
	}

	// Nil tool error
	_, err = reg.Register(nil)
	if err == nil {
		t.Fatalf("expected error on nil tool")
	}

	// Count and Has
	if reg.Count() != 2 {
		t.Fatalf("expected 2 tools, got %d", reg.Count())
	}
	if !reg.Has("t1") || !reg.Has("t2") {
		t.Fatalf("expected Has() to be true for t1 and t2")
	}
	if reg.Has("t3") {
		t.Fatalf("expected Has(t3) to be false")
	}

	// List preserves order
	list := reg.List()
	if len(list) != 2 || list[0].Name() != "t1" || list[1].Name() != "t2" {
		t.Fatalf("unexpected list: %v", list)
	}

	// Healthy tools filter (trip t2 to DEAD)
	failingTool := &tools.FuncTool{
		ToolName: "t2",
		Handler:  func(ctx context.Context, input any) (any, error) { return nil, errors.New("fail") },
	}
	// simulate t2 becoming dead
	p2Failing := tools.NewToolProxy(failingTool, tools.ToolProxyOptions{MaxConsecutiveFailures: 2})
	_, _ = p2Failing.Execute(context.Background(), nil)
	_, _ = p2Failing.Execute(context.Background(), nil)
	if p2Failing.Status() != tools.ToolStatusDead {
		t.Fatalf("expected p2Failing to be DEAD")
	}

	// Register a DEAD tool proxy into registry
	reg.Remove("t2")
	_, _ = reg.Register(p2Failing)

	healthy := reg.GetHealthyTools()
	if len(healthy) != 1 || healthy[0].Name() != "t1" {
		t.Fatalf("expected only t1 in healthy tools, got %d tools", len(healthy))
	}

	// Remove
	if !reg.Remove("t1") {
		t.Fatalf("expected Remove('t1') to succeed")
	}
	if reg.Remove("t1") {
		t.Fatalf("expected second Remove('t1') to return false")
	}

	// Clear
	reg.Clear()
	if reg.Count() != 0 {
		t.Fatalf("expected empty registry after Clear()")
	}
}

func TestToolRegistry_Concurrency(t *testing.T) {
	reg := tools.NewToolRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tool := &tools.FuncTool{
				ToolName: string(rune('a' + idx)),
				Handler:  func(ctx context.Context, input any) (any, error) { return idx, nil },
			}
			_, _ = reg.Register(tool)
			_ = reg.List()
			_ = reg.GetHealthyTools()
			_ = reg.Count()
		}(i)
	}
	wg.Wait()

	if reg.Count() != 30 {
		t.Fatalf("expected 30 registered tools, got %d", reg.Count())
	}
}
