package protocol

import (
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "strings"
)

type Logger interface {
    Printf(format string, args ...any)
}

type ApprovalDeniedError struct {
    Reason string
}

func (err ApprovalDeniedError) Error() string {
    if err.Reason == "" {
        return "Unity denied the direct MCP connection"
    }

    return fmt.Sprintf("Unity denied the direct MCP connection: %s", err.Reason)
}

type Client struct {
    rw     io.ReadWriteCloser
    reader *bufio.Reader
    writer *bufio.Writer
}

func NewClient(rw io.ReadWriteCloser) *Client {
    return &Client{
        rw:     rw,
        reader: bufio.NewReader(rw),
        writer: bufio.NewWriter(rw),
    }
}

func (client *Client) Close() error {
    return client.rw.Close()
}

func (client *Client) WaitForReady(ctx context.Context, logger Logger) error {
    for {
        rawMessage, err := client.readMessage(ctx)
        if err != nil {
            return err
        }

        var envelope map[string]any
        if err := json.Unmarshal([]byte(rawMessage), &envelope); err != nil {
            return fmt.Errorf("failed to parse Unity handshake message: %w", err)
        }

        messageType, _ := envelope["type"].(string)
        switch messageType {
        case "handshake":
            return nil
        case "approval_pending":
            if logger != nil {
                logger.Printf("Unity MCP connection approval is pending in Project Settings > AI > Unity MCP")
            }
        case "approval_denied":
            reason, _ := envelope["reason"].(string)
            return ApprovalDeniedError{Reason: reason}
        default:
            return fmt.Errorf("unexpected pre-handshake message type: %s", messageType)
        }
    }
}

func (client *Client) Call(ctx context.Context, request any, requestID string) ([]byte, error) {
    if err := client.writeMessage(ctx, request); err != nil {
        return nil, err
    }

    for {
        rawMessage, err := client.readMessage(ctx)
        if err != nil {
            return nil, err
        }

        var envelope map[string]any
        if err := json.Unmarshal([]byte(rawMessage), &envelope); err != nil {
            return nil, fmt.Errorf("failed to parse Unity response: %w", err)
        }

        messageType, _ := envelope["type"].(string)
        switch messageType {
        case "command_in_progress", "approval_pending":
            continue
        case "approval_denied":
            reason, _ := envelope["reason"].(string)
            return nil, ApprovalDeniedError{Reason: reason}
        }

        responseRequestID, _ := envelope["requestId"].(string)
        if requestID != "" && responseRequestID != "" && responseRequestID != requestID {
            continue
        }

        if _, ok := envelope["status"]; ok {
            return []byte(rawMessage), nil
        }
    }
}

func (client *Client) writeMessage(ctx context.Context, payload any) error {
    messageBytes, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to encode Unity request: %w", err)
    }

    return client.withContext(ctx, func() error {
        if _, err := client.writer.Write(messageBytes); err != nil {
            return fmt.Errorf("failed to write Unity request: %w", err)
        }

        if err := client.writer.WriteByte('\n'); err != nil {
            return fmt.Errorf("failed to write Unity request delimiter: %w", err)
        }

        if err := client.writer.Flush(); err != nil {
            return fmt.Errorf("failed to flush Unity request: %w", err)
        }

        return nil
    })
}

func (client *Client) readMessage(ctx context.Context) (string, error) {
    var line string
    err := client.withContext(ctx, func() error {
        rawLine, readErr := client.reader.ReadString('\n')
        if readErr != nil {
            if errors.Is(readErr, io.EOF) {
                return io.EOF
            }

            return fmt.Errorf("failed to read Unity response: %w", readErr)
        }

        line = strings.TrimSpace(rawLine)
        return nil
    })
    if err != nil {
        return "", err
    }

    return line, nil
}

func (client *Client) withContext(ctx context.Context, action func() error) error {
    if ctx == nil {
        return action()
    }

    done := make(chan error, 1)
    go func() {
        done <- action()
    }()

    select {
    case <-ctx.Done():
        _ = client.Close()
        return ctx.Err()
    case err := <-done:
        return err
    }
}
