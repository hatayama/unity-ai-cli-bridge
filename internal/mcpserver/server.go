package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

type BridgeFactory func(ctx context.Context) (unitybridge.Bridge, error)

type Server struct {
	bridgeFactory BridgeFactory
	bridge        unitybridge.Bridge
}

type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type responseEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(bridgeFactory BridgeFactory) *Server {
	return &Server{bridgeFactory: bridgeFactory}
}

func (server *Server) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)

	for {
		payload, err := readRPCPayload(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		responseBytes, err := server.handleRPC(ctx, payload)
		if err != nil {
			return err
		}

		if len(responseBytes) == 0 {
			continue
		}

		if err := writeRPCPayload(writer, responseBytes); err != nil {
			return err
		}
	}
}

func (server *Server) handleRPC(ctx context.Context, payload []byte) ([]byte, error) {
	var request requestEnvelope
	if err := json.Unmarshal(payload, &request); err != nil {
		response, marshalErr := json.Marshal(responseEnvelope{
			JSONRPC: "2.0",
			Error: &responseError{
				Code:    -32700,
				Message: "invalid JSON-RPC payload",
			},
		})
		if marshalErr != nil {
			return nil, marshalErr
		}
		return response, nil
	}

	if len(request.ID) == 0 {
		return nil, nil
	}

	switch request.Method {
	case "initialize":
		return json.Marshal(responseEnvelope{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result: map[string]any{
				"protocolVersion": resolveProtocolVersion(request.Params),
				"capabilities": map[string]any{
					"tools": map[string]any{
						"listChanged": false,
					},
				},
				"serverInfo": map[string]any{
					"name":    "unity-ai-cli",
					"version": "0.1.0",
				},
			},
		})
	case "ping":
		return json.Marshal(responseEnvelope{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result:  map[string]any{},
		})
	case "tools/list":
		bridge, err := server.ensureBridge(ctx)
		if err != nil {
			return json.Marshal(responseEnvelope{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error: &responseError{
					Code:    -32001,
					Message: err.Error(),
				},
			})
		}

		toolList, err := bridge.ListTools(ctx, "")
		if err != nil {
			return json.Marshal(responseEnvelope{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error: &responseError{
					Code:    -32002,
					Message: err.Error(),
				},
			})
		}

		tools := make([]map[string]any, 0, len(toolList.Tools))
		for _, tool := range toolList.Tools {
			toolEntry := map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
			}

			if len(tool.InputSchema) > 0 {
				var schema any
				if err := json.Unmarshal(tool.InputSchema, &schema); err == nil {
					toolEntry["inputSchema"] = schema
				}
			}

			tools = append(tools, toolEntry)
		}

		return json.Marshal(responseEnvelope{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result: map[string]any{
				"tools": tools,
			},
		})
	case "tools/call":
		bridge, err := server.ensureBridge(ctx)
		if err != nil {
			return json.Marshal(responseEnvelope{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error: &responseError{
					Code:    -32001,
					Message: err.Error(),
				},
			})
		}

		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return json.Marshal(responseEnvelope{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error: &responseError{
					Code:    -32602,
					Message: "invalid tools/call params",
				},
			})
		}

		result, err := bridge.CallTool(ctx, params.Name, params.Arguments)
		if err != nil {
			return json.Marshal(responseEnvelope{
				JSONRPC: "2.0",
				ID:      request.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{
							"type": "text",
							"text": err.Error(),
						},
					},
					"isError": true,
				},
			})
		}

		var structured any
		if len(result) > 0 {
			_ = json.Unmarshal(result, &structured)
		}

		resultText := "{}"
		if len(bytes.TrimSpace(result)) > 0 {
			resultText = string(result)
		}

		return json.Marshal(responseEnvelope{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": resultText,
					},
				},
				"structuredContent": structured,
			},
		})
	default:
		return json.Marshal(responseEnvelope{
			JSONRPC: "2.0",
			ID:      request.ID,
			Error: &responseError{
				Code:    -32601,
				Message: fmt.Sprintf("unsupported method %s", request.Method),
			},
		})
	}
}

func (server *Server) ensureBridge(ctx context.Context) (unitybridge.Bridge, error) {
	if server.bridge != nil {
		return server.bridge, nil
	}

	bridge, err := server.bridgeFactory(ctx)
	if err != nil {
		return nil, err
	}

	server.bridge = bridge
	return bridge, nil
}

func resolveProtocolVersion(payload json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}

	if err := json.Unmarshal(payload, &params); err == nil && params.ProtocolVersion != "" {
		return params.ProtocolVersion
	}

	return "2024-11-05"
}

func readRPCPayload(reader *bufio.Reader) ([]byte, error) {
	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}

		if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "Content-Length:"))
			if value == trimmed {
				value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(trimmed), "content-length:"))
			}

			parsedValue, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length header: %w", err)
			}
			contentLength = parsedValue
		}
	}

	if contentLength <= 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func writeRPCPayload(writer *bufio.Writer, payload []byte) error {
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}

	if _, err := writer.Write(payload); err != nil {
		return err
	}

	return writer.Flush()
}
