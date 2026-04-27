package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"bcli/internal/core/auth"
	"bcli/internal/core/profile"
)

const protocolVersion = "2025-11-25"

type Server struct {
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	auth     auth.Service
	profiles profile.Service
}

func NewServer(stdin io.Reader, stdout io.Writer, stderr io.Writer, authService auth.Service, profileService profile.Service) Server {
	return Server{
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		auth:     authService,
		profiles: profileService,
	}
}

func (s Server) Serve() error {
	scanner := bufio.NewScanner(s.stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(s.stdout)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if writeErr := encoder.Encode(errorResponse(nil, -32700, "parse error", err.Error())); writeErr != nil {
				return writeErr
			}
			continue
		}

		if req.ID == nil {
			// MCP notifications, such as notifications/initialized, do not expect responses.
			continue
		}

		result, rpcErr := s.handle(req)
		var err error
		if rpcErr != nil {
			err = encoder.Encode(errorResponse(req.ID, rpcErr.code, rpcErr.message, rpcErr.data))
		} else {
			err = encoder.Encode(successResponse(req.ID, result))
		}
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s Server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": s.tools()}, nil
	case "tools/call":
		return s.handleToolCall(req.Params)
	case "resources/list":
		return s.handleResourcesList()
	case "resources/read":
		return s.handleResourceRead(req.Params)
	case "resources/templates/list":
		return map[string]any{"resourceTemplates": s.resourceTemplates()}, nil
	case "prompts/list":
		return map[string]any{"prompts": s.prompts()}, nil
	case "prompts/get":
		return s.handlePromptGet(req.Params)
	default:
		return nil, methodNotFound(fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s Server) handleInitialize(params json.RawMessage) (any, *rpcError) {
	var init initializeParams
	if len(params) != 0 {
		if err := json.Unmarshal(params, &init); err != nil {
			return nil, invalidParams(err.Error())
		}
	}

	selectedVersion := protocolVersion
	if isSupportedProtocolVersion(init.ProtocolVersion) {
		selectedVersion = init.ProtocolVersion
	}

	return map[string]any{
		"protocolVersion": selectedVersion,
		"capabilities": map[string]any{
			"tools":     map[string]any{"listChanged": false},
			"resources": map[string]any{"subscribe": false, "listChanged": false},
			"prompts":   map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":        "bcli",
			"title":       "bcli local command center",
			"version":     "dev",
			"description": "Local profile, credential, and utility tools for bcli.",
		},
		"instructions": "Use bcli MCP tools for structured local profile management and small utilities. Credentials can be stored but are never returned by tools or resources.",
	}, nil
}

func isSupportedProtocolVersion(version string) bool {
	switch version {
	case "", protocolVersion, "2025-06-18", "2025-03-26", "2024-11-05":
		return true
	default:
		return false
	}
}

type request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type response struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      *json.RawMessage  `json:"id,omitempty"`
	Result  any               `json:"result,omitempty"`
	Error   *responseRPCError `json:"error,omitempty"`
}

type responseRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcError struct {
	code    int
	message string
	data    any
}

func successResponse(id *json.RawMessage, result any) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id *json.RawMessage, code int, message string, data any) response {
	return response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &responseRPCError{Code: code, Message: message, Data: data},
	}
}

func invalidParams(message string) *rpcError {
	return &rpcError{code: -32602, message: message}
}

func methodNotFound(message string) *rpcError {
	return &rpcError{code: -32601, message: message}
}

func internalError(message string) *rpcError {
	return &rpcError{code: -32603, message: message}
}
