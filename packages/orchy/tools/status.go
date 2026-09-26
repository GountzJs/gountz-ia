package tools

import (
	"time"
)

// ToolStatus represents the operational health status of a tool.
type ToolStatus string

const (
	ToolStatusHealthy  ToolStatus = "HEALTHY"
	ToolStatusDegraded ToolStatus = "DEGRADED"
	ToolStatusDead     ToolStatus = "DEAD"
)

// ToolMetrics records performance, success rates, and failure tracking for a tool.
type ToolMetrics struct {
	TotalCalls            int           `json:"totalCalls"`
	SuccessfulCalls       int           `json:"successfulCalls"`
	FailedCalls           int           `json:"failedCalls"`
	ConsecutiveFailures   int           `json:"consecutiveFailures"`
	AverageDuration       time.Duration `json:"averageDuration"`
	AverageDurationMs     float64       `json:"averageDurationMs"`
	LastExecutionDuration time.Duration `json:"lastExecutionDuration,omitempty"`
	LastExecutedAt        time.Time     `json:"lastExecutedAt,omitempty"`
	LastError             string        `json:"lastError,omitempty"`
}

// ToolSchema defines input requirements and structure following JSON Schema conventions.
type ToolSchema struct {
	Kind                 string         `json:"kind,omitempty"`
	Type                 string         `json:"type,omitempty"`
	Description          string         `json:"description,omitempty"`
	Properties           map[string]any `json:"properties,omitempty"`
	Required             []string       `json:"required,omitempty"`
	Items                any            `json:"items,omitempty"`
	AdditionalProperties *bool          `json:"additionalProperties,omitempty"`
	Extra                map[string]any `json:"extra,omitempty"`
}
