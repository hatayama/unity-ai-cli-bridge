package unitybridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/hatayama/unity-ai-cli-bridge/internal/discovery"
	"github.com/hatayama/unity-ai-cli-bridge/internal/protocol"
	"github.com/hatayama/unity-ai-cli-bridge/internal/transport"
)

type ToolInfo struct {
	Name         string          `json:"name"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Annotations  json.RawMessage `json:"annotations"`
}

type ToolListResult struct {
	Hash      string     `json:"hash"`
	Tools     []ToolInfo `json:"tools"`
	Unchanged bool       `json:"unchanged"`
}

type commandRequest struct {
	Type      string         `json:"type"`
	Params    map[string]any `json:"params,omitempty"`
	RequestID string         `json:"requestId"`
}

type commandResponse struct {
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result"`
	Error     string          `json:"error"`
	RequestID string          `json:"requestId"`
}

type Bridge interface {
	ListTools(ctx context.Context, hash string) (ToolListResult, error)
	CallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error)
	Close() error
}

type Client struct {
	protocolClient *protocol.Client
	logger         *log.Logger
	requestPrefix  string
	requestCounter uint64
}

var clientInstanceCounter atomic.Uint64

func Connect(ctx context.Context, resolved discovery.ResolvedBridge, clientName string) (*Client, error) {
	return connectWithDialer(ctx, resolved, clientName, transport.NewDialer())
}

func connectWithDialer(
	ctx context.Context,
	resolved discovery.ResolvedBridge,
	clientName string,
	dialer transport.Dialer,
) (*Client, error) {
	rawConnection, err := dialer.Dial(resolved.ConnectionInfo.ConnectionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Unity bridge %s: %w", resolved.ConnectionInfo.ConnectionPath, err)
	}

	protocolClient := protocol.NewClient(rawConnection)
	logger := log.New(os.Stderr, "", 0)
	if err := protocolClient.WaitForReady(ctx, logger); err != nil {
		_ = protocolClient.Close()
		return nil, err
	}

	client := &Client{
		protocolClient: protocolClient,
		logger:         logger,
		requestPrefix:  buildRequestPrefix(clientName),
	}

	if err := client.setClientInfo(ctx, clientName); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

func (client *Client) Close() error {
	return client.protocolClient.Close()
}

func (client *Client) ListTools(ctx context.Context, hash string) (ToolListResult, error) {
	params := map[string]any{}
	if hash != "" {
		params["hash"] = hash
	}

	response, err := client.call(ctx, "get_available_tools", params)
	if err != nil {
		return ToolListResult{}, err
	}

	if response.Status != "success" {
		return ToolListResult{}, errors.New(response.Error)
	}

	var result ToolListResult
	if len(response.Result) == 0 {
		return result, nil
	}

	if err := json.Unmarshal(response.Result, &result); err != nil {
		return ToolListResult{}, fmt.Errorf("failed to decode tool list: %w", err)
	}

	return result, nil
}

func (client *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}

	response, err := client.call(ctx, name, arguments)
	if err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, errors.New(response.Error)
	}

	return response.Result, nil
}

func (client *Client) setClientInfo(ctx context.Context, clientName string) error {
	_, err := client.call(ctx, "set_client_info", map[string]any{
		"name":    clientName,
		"title":   clientName,
		"version": "0.1.0",
	})
	return err
}

func (client *Client) call(ctx context.Context, commandType string, params map[string]any) (commandResponse, error) {
	requestID := client.nextRequestID()
	request := commandRequest{
		Type:      commandType,
		Params:    params,
		RequestID: requestID,
	}

	rawResponse, err := client.protocolClient.Call(ctx, request, requestID)
	if err != nil {
		return commandResponse{}, err
	}

	var response commandResponse
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		return commandResponse{}, fmt.Errorf("failed to decode Unity response: %w", err)
	}

	return response, nil
}

func (client *Client) nextRequestID() string {
	requestNumber := atomic.AddUint64(&client.requestCounter, 1)
	return fmt.Sprintf("%s-%d", client.requestPrefix, requestNumber)
}

func buildRequestPrefix(clientName string) string {
	clientInstance := clientInstanceCounter.Add(1)
	return fmt.Sprintf("%s-%d-%d", clientName, time.Now().UnixNano(), clientInstance)
}
