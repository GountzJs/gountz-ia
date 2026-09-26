package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Default circuit breaker values.
const (
	DefaultMaxConsecutiveFailures = 5
	DefaultTimeout                = 30 * time.Second
)

// ToolProxyOptions configures the resilience and circuit breaker settings of a ToolProxy.
type ToolProxyOptions struct {
	MaxConsecutiveFailures int
	Timeout                time.Duration
}

// ToolProxy wraps a Tool with metrics tracking and circuit breaker protection (Honest Kernel).
type ToolProxy struct {
	mu                     sync.RWMutex
	targetTool             Tool
	status                 ToolStatus
	maxConsecutiveFailures int
	timeout                time.Duration
	metrics                ToolMetrics
}

// NewToolProxy creates a new proxy wrapping targetTool with the specified options.
func NewToolProxy(targetTool Tool, opts ...ToolProxyOptions) *ToolProxy {
	maxFailures := DefaultMaxConsecutiveFailures
	timeout := DefaultTimeout

	if len(opts) > 0 {
		if opts[0].MaxConsecutiveFailures > 0 {
			maxFailures = opts[0].MaxConsecutiveFailures
		}
		if opts[0].Timeout > 0 {
			timeout = opts[0].Timeout
		}
	}

	return &ToolProxy{
		targetTool:             targetTool,
		status:                 ToolStatusHealthy,
		maxConsecutiveFailures: maxFailures,
		timeout:                timeout,
		metrics: ToolMetrics{
			TotalCalls:          0,
			SuccessfulCalls:     0,
			FailedCalls:         0,
			ConsecutiveFailures: 0,
		},
	}
}

func (p *ToolProxy) recordSuccess(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.TotalCalls++
	p.metrics.SuccessfulCalls++
	p.metrics.ConsecutiveFailures = 0
	p.metrics.LastExecutionDuration = duration
	p.metrics.LastExecutedAt = time.Now()
	p.updateAverageDuration(duration)

	if p.status == ToolStatusDegraded {
		p.status = ToolStatusHealthy
	}
}

func (p *ToolProxy) recordFailure(duration time.Duration, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.TotalCalls++
	p.metrics.FailedCalls++
	p.metrics.ConsecutiveFailures++
	p.metrics.LastExecutionDuration = duration
	p.metrics.LastError = err.Error()
	p.metrics.LastExecutedAt = time.Now()
	p.updateAverageDuration(duration)

	if p.metrics.ConsecutiveFailures >= p.maxConsecutiveFailures {
		p.status = ToolStatusDead
	} else if p.metrics.ConsecutiveFailures >= 2 {
		p.status = ToolStatusDegraded
	}
}

func (p *ToolProxy) updateAverageDuration(newDuration time.Duration) {
	total := p.metrics.TotalCalls
	if total <= 1 {
		p.metrics.AverageDuration = newDuration
		p.metrics.AverageDurationMs = float64(newDuration.Milliseconds())
		return
	}

	// Moving average calculation
	prevTotal := total - 1
	prevAvgNanos := p.metrics.AverageDuration.Nanoseconds()
	newAvgNanos := (prevAvgNanos*int64(prevTotal) + newDuration.Nanoseconds()) / int64(total)
	p.metrics.AverageDuration = time.Duration(newAvgNanos)
	p.metrics.AverageDurationMs = float64(p.metrics.AverageDuration.Milliseconds())
}

// ResetCircuitBreaker manually resets the tool status to HEALTHY and zeroes consecutive failures.
func (p *ToolProxy) ResetCircuitBreaker() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = ToolStatusHealthy
	p.metrics.ConsecutiveFailures = 0
}

// Execute invokes the wrapped tool under circuit breaker and timeout supervision.
func (p *ToolProxy) Execute(ctx context.Context, input any) (any, error) {
	p.mu.RLock()
	currentStatus := p.status
	consecutiveFailures := p.metrics.ConsecutiveFailures
	p.mu.RUnlock()

	if currentStatus == ToolStatusDead {
		return nil, fmt.Errorf("[ToolProxy] circuit breaker tripped for tool %q: status is DEAD after %d consecutive failures", p.Name(), consecutiveFailures)
	}

	execCtx := ctx
	var cancel context.CancelFunc
	if p.timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	startTime := time.Now()

	// Execute with panic recovery
	type execResult struct {
		val any
		err error
	}

	resCh := make(chan execResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resCh <- execResult{val: nil, err: fmt.Errorf("panic in tool %q: %v", p.Name(), r)}
			}
		}()
		val, err := p.targetTool.Execute(execCtx, input)
		resCh <- execResult{val: val, err: err}
	}()

	select {
	case <-execCtx.Done():
		duration := time.Since(startTime)
		err := execCtx.Err()
		p.recordFailure(duration, err)
		return nil, fmt.Errorf("[ToolProxy] execution of tool %q failed: %w", p.Name(), err)
	case res := <-resCh:
		duration := time.Since(startTime)
		if res.err != nil {
			p.recordFailure(duration, res.err)
			return nil, res.err
		}
		p.recordSuccess(duration)
		return res.val, nil
	}
}

// Name returns the name of the wrapped tool.
func (p *ToolProxy) Name() string {
	return p.targetTool.Name()
}

// Description returns the description of the wrapped tool.
func (p *ToolProxy) Description() string {
	return p.targetTool.Description()
}

// Schema returns the schema of the wrapped tool.
func (p *ToolProxy) Schema() ToolSchema {
	return p.targetTool.Schema()
}

// Status returns the current circuit breaker status of the tool.
func (p *ToolProxy) Status() ToolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

// Metrics returns a snapshot copy of the tool's execution metrics.
func (p *ToolProxy) Metrics() ToolMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metrics
}

// RawTool returns the underlying Tool instance.
func (p *ToolProxy) RawTool() Tool {
	return p.targetTool
}
