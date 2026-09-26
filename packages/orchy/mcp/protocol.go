package mcp

import "encoding/json"

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// JSONRPCRequest represents a JSON-RPC 2.0 request or notification.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError defines the error object within a JSON-RPC 2.0 response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ImplementationInfo holds name and version metadata for a client or server.
type ImplementationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams represents incoming parameters for the "initialize" MCP method.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    map[string]any     `json:"capabilities,omitempty"`
	ClientInfo      ImplementationInfo `json:"clientInfo,omitempty"`
}

// ServerCapabilities defines features supported by the MCP server.
type ServerCapabilities struct {
	Tools map[string]any `json:"tools,omitempty"`
}

// InitializeResult represents the response for the "initialize" MCP method.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ImplementationInfo `json:"serverInfo"`
}

// ToolsListResult represents the output of the "tools/list" MCP method.
type ToolsListResult struct {
	Tools []McpToolDefinition `json:"tools"`
}

// ToolCallParams represents incoming arguments for the "tools/call" MCP method.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolCallContent defines a content element returned by a tool execution.
type ToolCallContent struct {
	Type string `json:"type"` // e.g. "text"
	Text string `json:"text"`
}

// ToolCallResult represents the output of the "tools/call" MCP method.
type ToolCallResult struct {
	Content []ToolCallContent `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}
