package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// ToolExecutor executes tools by name with input arguments.
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, name string, input any) (any, error)
}

// Server implements an MCP stdio server responding to JSON-RPC 2.0 requests.
type Server struct {
	manifest   *ManifestGenerator
	executor   ToolExecutor
	in         io.Reader
	out        io.Writer
	writeMu    sync.Mutex
	serverInfo ImplementationInfo
}

// NewServer creates a new MCP server.
func NewServer(manifest *ManifestGenerator, executor ToolExecutor, in io.Reader, out io.Writer) *Server {
	return &Server{
		manifest: manifest,
		executor: executor,
		in:       in,
		out:      out,
		serverInfo: ImplementationInfo{
			Name:    "orchy-mcp-server",
			Version: "1.0.0",
		},
	}
}

// SetServerInfo sets custom name and version for the server info response.
func (s *Server) SetServerInfo(name, version string) {
	s.serverInfo = ImplementationInfo{
		Name:    name,
		Version: version,
	}
}

// HandleRequest processes a single JSON-RPC 2.0 request payload and returns the response bytes.
// If the request was a notification (no ID), it returns nil, nil.
func (s *Server) HandleRequest(ctx context.Context, raw []byte) ([]byte, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &JSONRPCError{
				Code:    CodeParseError,
				Message: fmt.Sprintf("Parse error: %v", err),
			},
		}
		return json.Marshal(resp)
	}

	if req.JSONRPC != "2.0" {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    CodeInvalidRequest,
				Message: "Invalid Request: jsonrpc must be '2.0'",
			},
		}
		return json.Marshal(resp)
	}

	// Notifications have no ID
	isNotification := req.ID == nil

	var result any
	var rpcErr *JSONRPCError

	switch req.Method {
	case "initialize":
		result = InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: ServerCapabilities{
				Tools: map[string]any{},
			},
			ServerInfo: s.serverInfo,
		}

	case "notifications/initialized":
		return nil, nil

	case "ping":
		result = map[string]any{}

	case "tools/list":
		var toolsList []McpToolDefinition
		if s.manifest != nil {
			toolsList = s.manifest.GenerateMcpTools()
		}
		result = ToolsListResult{
			Tools: toolsList,
		}

	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			rpcErr = &JSONRPCError{
				Code:    CodeInvalidParams,
				Message: fmt.Sprintf("Invalid params for tools/call: %v", err),
			}
		} else if s.executor == nil {
			rpcErr = &JSONRPCError{
				Code:    CodeInternalError,
				Message: "No tool executor configured on MCP server",
			}
		} else {
			out, err := s.executor.ExecuteTool(ctx, params.Name, params.Arguments)
			if err != nil {
				// According to MCP specification, tool execution failures return ToolCallResult with IsError = true
				result = ToolCallResult{
					IsError: true,
					Content: []ToolCallContent{
						{
							Type: "text",
							Text: fmt.Sprintf("Error executing tool %q: %v", params.Name, err),
						},
					},
				}
			} else {
				var text string
				if str, ok := out.(string); ok {
					text = str
				} else {
					bytes, mErr := json.Marshal(out)
					if mErr != nil {
						text = fmt.Sprintf("%v", out)
					} else {
						text = string(bytes)
					}
				}
				result = ToolCallResult{
					IsError: false,
					Content: []ToolCallContent{
						{
							Type: "text",
							Text: text,
						},
					},
				}
			}
		}

	default:
		rpcErr = &JSONRPCError{
			Code:    CodeMethodNotFound,
			Message: fmt.Sprintf("Method %q not found", req.Method),
		}
	}

	if isNotification {
		return nil, nil
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	}

	return json.Marshal(resp)
}

// Serve reads JSON-RPC 2.0 messages from the input stream line by line and writes responses to the output stream.
func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		respBytes, err := s.HandleRequest(ctx, line)
		if err != nil {
			return err
		}

		if respBytes != nil {
			s.writeMu.Lock()
			if _, wErr := s.out.Write(respBytes); wErr != nil {
				s.writeMu.Unlock()
				return wErr
			}
			if _, wErr := s.out.Write([]byte("\n")); wErr != nil {
				s.writeMu.Unlock()
				return wErr
			}
			s.writeMu.Unlock()
		}
	}

	return scanner.Err()
}
