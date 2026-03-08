package unitybridge

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hatayama/unity-ai-cli-bridge/internal/discovery"
)

func TestConnectListToolsAndCallTool(t *testing.T) {
	dialer, shutdown := startFakeBridge(t, func(connection net.Conn) {
		sendLine(t, connection, `{"type":"handshake","protocol":"unity-mcp","version":"2.0"}`)

		setClientInfo := readJSONLine(t, connection)
		requestID, _ := setClientInfo["requestId"].(string)
		sendLine(t, connection, `{"status":"success","result":{"message":"ok"},"requestId":"`+requestID+`"}`)

		getTools := readJSONLine(t, connection)
		requestID, _ = getTools["requestId"].(string)
		sendLine(t, connection, `{"status":"success","result":{"hash":"abc","tools":[{"name":"Unity.ReadConsole","title":"Read Console","description":"Reads Unity console output","inputSchema":{"type":"object","properties":{}},"outputSchema":{"type":"object"}}]},"requestId":"`+requestID+`"}`)

		callTool := readJSONLine(t, connection)
		requestID, _ = callTool["requestId"].(string)
		sendLine(t, connection, `{"type":"command_in_progress","message":"Command execution in progress"}`)
		sendLine(t, connection, `{"status":"success","result":{"entries":[{"message":"hello"}]},"requestId":"`+requestID+`"}`)
	})
	defer shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := connectWithDialer(ctx, discovery.ResolvedBridge{
		ConnectionInfo: discovery.ConnectionInfo{
			ConnectionPath: "test-bridge",
		},
	}, "unity-ai-cli-test", dialer)
	if err != nil {
		t.Fatalf("expected connection to succeed, got error: %v", err)
	}
	defer func() { _ = client.Close() }()

	tools, err := client.ListTools(ctx, "")
	if err != nil {
		t.Fatalf("expected tool discovery to succeed, got error: %v", err)
	}

	if len(tools.Tools) != 1 || tools.Tools[0].Name != "Unity.ReadConsole" {
		t.Fatalf("unexpected tool list: %#v", tools.Tools)
	}

	result, err := client.CallTool(ctx, "Unity.ReadConsole", map[string]any{})
	if err != nil {
		t.Fatalf("expected tool call to succeed, got error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("failed to decode tool result: %v", err)
	}

	if _, ok := payload["entries"]; !ok {
		t.Fatalf("expected entries field, got %#v", payload)
	}
}

func TestConnectFailsWhenApprovalIsDenied(t *testing.T) {
	dialer, shutdown := startFakeBridge(t, func(connection net.Conn) {
		sendLine(t, connection, `{"type":"approval_denied","reason":"manual denial"}`)
	})
	defer shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := connectWithDialer(ctx, discovery.ResolvedBridge{
		ConnectionInfo: discovery.ConnectionInfo{
			ConnectionPath: "test-bridge",
		},
	}, "unity-ai-cli-test", dialer)
	if err == nil {
		t.Fatal("expected approval denial error")
	}
}

func TestNextRequestIDIsUniqueAcrossClients(t *testing.T) {
	firstClient := &Client{
		requestPrefix: buildRequestPrefix("unity-ai-cli-test"),
	}
	secondClient := &Client{
		requestPrefix: buildRequestPrefix("unity-ai-cli-test"),
	}

	firstRequestID := firstClient.nextRequestID()
	secondRequestID := secondClient.nextRequestID()

	if firstRequestID == secondRequestID {
		t.Fatalf("expected unique request IDs across clients, got %s", firstRequestID)
	}
}

type fakeDialer struct {
	serverConnection net.Conn
}

func (dialer fakeDialer) Dial(path string) (io.ReadWriteCloser, error) {
	return dialer.serverConnection, nil
}

func startFakeBridge(t *testing.T, handler func(connection net.Conn)) (fakeDialer, func()) {
	t.Helper()

	clientConnection, serverConnection := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = clientConnection.Close() }()
		handler(clientConnection)
	}()

	shutdown := func() {
		_ = serverConnection.Close()
		<-done
	}

	return fakeDialer{serverConnection: serverConnection}, shutdown
}

func sendLine(t *testing.T, connection net.Conn, line string) {
	t.Helper()
	if _, err := connection.Write([]byte(line + "\n")); err != nil {
		t.Fatalf("failed to write fake bridge message: %v", err)
	}
}

func readJSONLine(t *testing.T, connection net.Conn) map[string]any {
	t.Helper()

	buffer := make([]byte, 4096)
	count, err := connection.Read(buffer)
	if err != nil {
		t.Fatalf("failed to read fake bridge request: %v", err)
	}

	payload := make(map[string]any)
	if err := json.Unmarshal(buffer[:count-1], &payload); err != nil {
		t.Fatalf("failed to parse fake bridge request: %v", err)
	}

	return payload
}
