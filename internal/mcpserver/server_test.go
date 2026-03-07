package mcpserver

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "testing"

    "github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

type fakeBridge struct{}

func (fakeBridge) ListTools(ctx context.Context, hash string) (unitybridge.ToolListResult, error) {
    return unitybridge.ToolListResult{
        Hash: "abc",
        Tools: []unitybridge.ToolInfo{
            {
                Name:        "Unity.ReadConsole",
                Description: "Reads Unity console output",
                InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
            },
        },
    }, nil
}

func (fakeBridge) CallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
    return json.RawMessage(`{"entries":[{"message":"hello"}]}`), nil
}

func (fakeBridge) Close() error {
    return nil
}

func TestServerHandlesToolsListAndCall(t *testing.T) {
    server := New(func(ctx context.Context) (unitybridge.Bridge, error) {
        return fakeBridge{}, nil
    })

    input := bytes.NewBuffer(nil)
    output := bytes.NewBuffer(nil)

    writeRequest(input, map[string]any{
        "jsonrpc": "2.0",
        "id":      1,
        "method":  "initialize",
        "params": map[string]any{
            "protocolVersion": "2024-11-05",
        },
    })
    writeRequest(input, map[string]any{
        "jsonrpc": "2.0",
        "id":      2,
        "method":  "tools/list",
        "params":  map[string]any{},
    })
    writeRequest(input, map[string]any{
        "jsonrpc": "2.0",
        "id":      3,
        "method":  "tools/call",
        "params": map[string]any{
            "name":      "Unity.ReadConsole",
            "arguments": map[string]any{},
        },
    })

    if err := server.Serve(context.Background(), input, output); err != nil {
        t.Fatalf("expected MCP server to succeed, got error: %v", err)
    }

    responses := readResponses(t, output.Bytes())
    if len(responses) != 3 {
        t.Fatalf("expected 3 responses, got %d", len(responses))
    }

    toolListResult := responses[1]["result"].(map[string]any)
    tools := toolListResult["tools"].([]any)
    if len(tools) != 1 {
        t.Fatalf("expected one tool, got %#v", tools)
    }

    toolCallResult := responses[2]["result"].(map[string]any)
    if _, ok := toolCallResult["structuredContent"]; !ok {
        t.Fatalf("expected structuredContent in tools/call result, got %#v", toolCallResult)
    }
}

func writeRequest(buffer *bytes.Buffer, payload map[string]any) {
    body, _ := json.Marshal(payload)
    _, _ = buffer.WriteString("Content-Length: ")
    _, _ = buffer.WriteString(jsonLength(body))
    _, _ = buffer.WriteString("\r\n\r\n")
    _, _ = buffer.Write(body)
}

func jsonLength(body []byte) string {
    return string([]byte(fmtInt(len(body))))
}

func fmtInt(value int) string {
    if value == 0 {
        return "0"
    }

    digits := make([]byte, 0, 10)
    remaining := value
    for remaining > 0 {
        digits = append([]byte{byte('0' + remaining%10)}, digits...)
        remaining /= 10
    }
    return string(digits)
}

func readResponses(t *testing.T, payload []byte) []map[string]any {
    t.Helper()

    reader := bytes.NewBuffer(payload)
    responses := make([]map[string]any, 0, 4)
    for reader.Len() > 0 {
        header, err := reader.ReadString('\n')
        if err != nil {
            t.Fatalf("failed to read response header: %v", err)
        }
        if header == "\r\n" {
            continue
        }

        var length int
        if _, err := fmt.Sscanf(header, "Content-Length: %d\r\n", &length); err != nil {
            t.Fatalf("failed to parse Content-Length: %v", err)
        }

        blankLine := make([]byte, 2)
        if _, err := reader.Read(blankLine); err != nil {
            t.Fatalf("failed to read response separator: %v", err)
        }

        body := make([]byte, length)
        if _, err := reader.Read(body); err != nil {
            t.Fatalf("failed to read response body: %v", err)
        }

        decoded := make(map[string]any)
        if err := json.Unmarshal(body, &decoded); err != nil {
            t.Fatalf("failed to decode response body: %v", err)
        }

        responses = append(responses, decoded)
    }

    return responses
}
