package events_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"gz-ia/packages/orchy/events"
)

func TestEventBus_PubSub(t *testing.T) {
	bus := events.NewEventBus()
	ctx := context.Background()

	var counter int32
	sub1 := bus.Subscribe("test:event", func(ctx context.Context, payload any) error {
		atomic.AddInt32(&counter, 1)
		return nil
	})
	sub2 := bus.Subscribe("test:event", func(ctx context.Context, payload any) error {
		atomic.AddInt32(&counter, 2)
		return nil
	})

	err := bus.Publish(ctx, "test:event", "hello")
	if err != nil {
		t.Fatalf("unexpected error publishing: %v", err)
	}

	if atomic.LoadInt32(&counter) != 3 {
		t.Fatalf("expected counter to be 3, got %d", counter)
	}

	// Dispose sub1
	sub1.Dispose()

	err = bus.Publish(ctx, "test:event", "world")
	if err != nil {
		t.Fatalf("unexpected error publishing: %v", err)
	}

	if atomic.LoadInt32(&counter) != 5 {
		t.Fatalf("expected counter to be 5, got %d", counter)
	}

	// Dispose sub2
	sub2.Dispose()

	err = bus.Publish(ctx, "test:event", "again")
	if err != nil {
		t.Fatalf("unexpected error publishing: %v", err)
	}

	if atomic.LoadInt32(&counter) != 5 {
		t.Fatalf("expected counter to remain 5, got %d", counter)
	}
}

func TestEventBus_Publish_ContextCancellation(t *testing.T) {
	bus := events.NewEventBus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bus.Subscribe("test:cancel", func(ctx context.Context, payload any) error {
		return nil
	})

	err := bus.Publish(ctx, "test:cancel", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestEventBus_RPC(t *testing.T) {
	bus := events.NewEventBus()
	ctx := context.Background()

	// Request with no handler
	_, err := bus.Request(ctx, "calc:add", []int{1, 2})
	if err == nil {
		t.Fatalf("expected error for unregistered topic")
	}

	// Register handler
	disp, err := bus.Respond("calc:add", func(ctx context.Context, payload any) (any, error) {
		nums, ok := payload.([]int)
		if !ok || len(nums) != 2 {
			return nil, errors.New("invalid payload")
		}
		return nums[0] + nums[1], nil
	})
	if err != nil {
		t.Fatalf("failed to register RPC responder: %v", err)
	}

	// Register duplicate should fail
	_, err = bus.Respond("calc:add", func(ctx context.Context, payload any) (any, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatalf("expected error when registering duplicate RPC responder")
	}

	// Successful request
	res, err := bus.Request(ctx, "calc:add", []int{3, 4})
	if err != nil {
		t.Fatalf("unexpected RPC error: %v", err)
	}
	if res.(int) != 7 {
		t.Fatalf("expected 7, got %v", res)
	}

	// Dispose responder
	disp.Dispose()

	_, err = bus.Request(ctx, "calc:add", []int{3, 4})
	if err == nil {
		t.Fatalf("expected error after disposing RPC responder")
	}
}

func TestEventBus_RPC_ContextTimeout(t *testing.T) {
	bus := events.NewEventBus()

	_, _ = bus.Respond("slow:task", func(ctx context.Context, payload any) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return "done", nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := bus.Request(ctx, "slow:task", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestEventBus_Clear(t *testing.T) {
	bus := events.NewEventBus()

	bus.Subscribe("event", func(ctx context.Context, payload any) error { return nil })
	_, _ = bus.Respond("topic", func(ctx context.Context, payload any) (any, error) { return "ok", nil })

	bus.Clear()

	err := bus.Publish(context.Background(), "event", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = bus.Request(context.Background(), "topic", nil)
	if err == nil {
		t.Fatalf("expected error requesting cleared topic")
	}
}
