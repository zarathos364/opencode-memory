// Package mcp serves the memory store over the MCP stdio transport. Messages
// are newline-delimited JSON-RPC 2.0, hand-rolled with the standard library
// because the surface needed here is small: no new dependency.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// protocolVersion is the MCP revision this server speaks.
const protocolVersion = "2025-11-25"

// JSON-RPC error codes.
const (
	errParse          = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
)

// Server serves one store. The root is resolved once, before Run, so every
// tool call works on the same store.
type Server struct {
	Root    string
	Version string
}

// Serve runs one session against an already resolved root.
func Serve(in io.Reader, out io.Writer, root, version string) error {
	return (&Server{Root: root, Version: version}).Run(in, out)
}

// Run serves requests until the input ends. One line holds one message. A
// notification gets no reply, and malformed input gets a JSON-RPC error
// object, never a crash.
func (s *Server) Run(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	writer := bufio.NewWriter(out)
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if reply := s.handle(bytes.TrimSpace(line)); reply != nil {
				if _, werr := writer.Write(append(reply, '\n')); werr != nil {
					return fmt.Errorf("write reply: %w", werr)
				}
				if ferr := writer.Flush(); ferr != nil {
					return fmt.Errorf("write reply: %w", ferr)
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read request: %w", err)
		}
	}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (s *Server) handle(line []byte) []byte {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return errorReply(nil, errParse, "invalid request")
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return errorReply(req.ID, errInvalidRequest, "invalid request")
	}
	// A message without an id is a notification and gets no reply.
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil
	}
	switch req.Method {
	case "ping":
		return resultReply(req.ID, map[string]any{})
	case "initialize":
		return resultReply(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "opencode-memory", "version": s.Version},
		})
	case "tools/list":
		return resultReply(req.ID, map[string]any{"tools": toolDefinitions()})
	case "tools/call":
		return s.call(req.ID, req.Params)
	default:
		return errorReply(req.ID, errMethodNotFound, fmt.Sprintf("unknown method %q", req.Method))
	}
}

func resultReply(id json.RawMessage, result any) []byte {
	reply, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		return errorReply(id, errParse, "encode reply")
	}
	return reply
}

func errorReply(id json.RawMessage, code int, message string) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	reply, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	})
	if err != nil {
		return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"encode reply"}}`)
	}
	return reply
}
