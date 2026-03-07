package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	liveE2EEnvVar          = "UNITY_AI_CLI_E2E"
	liveE2EBinaryEnvVar    = "UNITY_AI_CLI_E2E_BIN"
	defaultLiveE2ETool     = "Unity_GetConsoleLogs"
	defaultLiveE2EJSONArgs = `{"maxEntries":2,"includeStackTrace":false}`
	defaultLiveE2EBinary   = "/tmp/unity-ai-cli"
)

func TestLiveUnityEndToEnd(t *testing.T) {
	if strings.TrimSpace(liveE2EEnvValue(t, liveE2EEnvVar)) == "" {
		t.Skipf("%s is not set", liveE2EEnvVar)
	}

	toolName := defaultLiveE2ETool
	binaryPath := resolveLiveBinaryPath(t)

	statusOutput := runCLICommand(t, binaryPath, nil, "bridge", "status", "--json")
	statusPayload := decodeJSONObject(t, statusOutput)
	connectionPayload := decodeObjectField(t, statusPayload, "connection")
	connectionPath := stringField(t, connectionPayload, "connection_path")
	if strings.TrimSpace(connectionPath) == "" {
		t.Fatal("expected bridge status to include connection_path")
	}

	listOutput := runCLICommand(t, binaryPath, nil, "tools", "list", "--json")
	listPayload := decodeJSONObject(t, listOutput)
	toolsValue, ok := listPayload["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %#v", listPayload["tools"])
	}

	foundTool := false
	for _, rawTool := range toolsValue {
		toolPayload, ok := rawTool.(map[string]any)
		if !ok {
			t.Fatalf("expected tool object, got %#v", rawTool)
		}

		if stringField(t, toolPayload, "name") == toolName {
			foundTool = true
			break
		}
	}

	if !foundTool {
		t.Fatalf("expected to find tool %s in %#v", toolName, toolsValue)
	}

	describeOutput := runCLICommand(t, binaryPath, nil, "tools", "describe", toolName, "--json")
	describePayload := decodeJSONObject(t, describeOutput)
	if stringField(t, describePayload, "name") != toolName {
		t.Fatalf("expected describe output for %s, got %#v", toolName, describePayload)
	}

	callOutput := runCLICommand(t, binaryPath, nil, "tools", "call", toolName, "--json-args", defaultLiveE2EJSONArgs)
	callPayload := decodeJSONObject(t, callOutput)
	successValue, ok := callPayload["success"].(bool)
	if !ok {
		t.Fatalf("expected success field, got %#v", callPayload["success"])
	}

	if !successValue {
		t.Fatalf("expected successful tool call, got %#v", callPayload)
	}

	dataPayload := decodeObjectField(t, callPayload, "data")
	if _, ok := dataPayload["logs"]; !ok {
		t.Fatalf("expected console log payload, got %#v", dataPayload)
	}

	mcpInput := bytes.NewBuffer(nil)
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
		},
	})
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": toolName,
			"arguments": map[string]any{
				"maxEntries":        2,
				"includeStackTrace": false,
			},
		},
	})

	mcpOutput := runCLICommand(t, binaryPath, mcpInput.Bytes(), "serve-mcp")
	mcpResponses := readRPCResponses(t, mcpOutput)
	if len(mcpResponses) != 3 {
		t.Fatalf("expected 3 MCP responses, got %d", len(mcpResponses))
	}

	mcpToolsResult := decodeArrayField(t, decodeObjectField(t, mcpResponses[1], "result"), "tools")
	if len(mcpToolsResult) == 0 {
		t.Fatal("expected MCP tools/list to return at least one tool")
	}

	mcpCallResult := decodeObjectField(t, mcpResponses[2], "result")
	structuredContent := decodeObjectField(t, mcpCallResult, "structuredContent")
	structuredSuccess, ok := structuredContent["success"].(bool)
	if !ok {
		t.Fatalf("expected structuredContent.success, got %#v", structuredContent["success"])
	}

	if !structuredSuccess {
		t.Fatalf("expected successful MCP tool call, got %#v", structuredContent)
	}
}

func liveE2EEnvValue(t *testing.T, envVar string) string {
	t.Helper()
	return strings.TrimSpace(os.Getenv(envVar))
}

func resolveLiveBinaryPath(t *testing.T) string {
	t.Helper()

	binaryPath := strings.TrimSpace(os.Getenv(liveE2EBinaryEnvVar))
	if binaryPath == "" {
		binaryPath = defaultLiveE2EBinary
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("failed to access E2E binary %s: %v", binaryPath, err)
	}

	if info.IsDir() {
		t.Fatalf("expected E2E binary path, got directory %s", binaryPath)
	}

	return binaryPath
}

func runCLICommand(t *testing.T, binaryPath string, stdin []byte, args ...string) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, binaryPath, args...)
	command.Stdin = bytes.NewReader(stdin)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	command.Stdout = stdout
	command.Stderr = stderr

	if err := command.Run(); err != nil {
		t.Fatalf("command %q failed: %v: %s", strings.Join(append([]string{binaryPath}, args...), " "), err, stderr.String())
	}

	return stdout.Bytes()
}

func decodeJSONObject(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to decode JSON object %q: %v", string(payload), err)
	}

	return decoded
}

func decodeObjectField(t *testing.T, payload map[string]any, fieldName string) map[string]any {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	decoded, ok := fieldValue.(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object, got %#v", fieldName, fieldValue)
	}

	return decoded
}

func stringField(t *testing.T, payload map[string]any, fieldName string) string {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	stringValue, ok := fieldValue.(string)
	if !ok {
		t.Fatalf("expected %s to be a string, got %#v", fieldName, fieldValue)
	}

	return stringValue
}

func decodeArrayField(t *testing.T, payload map[string]any, fieldName string) []any {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	arrayValue, ok := fieldValue.([]any)
	if !ok {
		t.Fatalf("expected %s to be an array, got %#v", fieldName, fieldValue)
	}

	return arrayValue
}

func writeRPCRequest(t *testing.T, buffer *bytes.Buffer, payload map[string]any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to encode RPC payload: %v", err)
	}

	if _, err := fmt.Fprintf(buffer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		t.Fatalf("failed to write RPC header: %v", err)
	}

	if _, err := buffer.Write(body); err != nil {
		t.Fatalf("failed to write RPC body: %v", err)
	}
}

func readRPCResponses(t *testing.T, payload []byte) []map[string]any {
	t.Helper()

	reader := bytes.NewBuffer(payload)
	responses := make([]map[string]any, 0, 3)
	for reader.Len() > 0 {
		header, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read RPC header: %v", err)
		}

		if header == "\r\n" {
			continue
		}

		var contentLength int
		if _, err := fmt.Sscanf(header, "Content-Length: %d\r\n", &contentLength); err != nil {
			t.Fatalf("failed to parse Content-Length from %q: %v", header, err)
		}

		separator := make([]byte, 2)
		if _, err := reader.Read(separator); err != nil {
			t.Fatalf("failed to read RPC separator: %v", err)
		}

		body := make([]byte, contentLength)
		if _, err := reader.Read(body); err != nil {
			t.Fatalf("failed to read RPC body: %v", err)
		}

		responses = append(responses, decodeJSONObject(t, body))
	}

	return responses
}
