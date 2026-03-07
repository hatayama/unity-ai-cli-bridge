package cli

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "github.com/hatayama/unity-ai-cli-bridge/internal/discovery"
    "github.com/hatayama/unity-ai-cli-bridge/internal/mcpserver"
    "github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

const clientName = "unity-ai-cli"

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
    if len(args) == 0 {
        printUsage(stderr)
        return 1
    }

    switch args[0] {
    case "bridge":
        return runBridge(args[1:], stdout, stderr)
    case "tools":
        return runTools(args[1:], stdout, stderr)
    case "serve-mcp":
        return runServeMcp(args[1:], stdin, stdout, stderr)
    default:
        fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
        printUsage(stderr)
        return 1
    }
}

func runBridge(args []string, stdout io.Writer, stderr io.Writer) int {
    if len(args) == 0 || args[0] != "status" {
        fmt.Fprintln(stderr, "bridge supports only the status subcommand")
        return 1
    }

    flags := flag.NewFlagSet("bridge status", flag.ContinueOnError)
    flags.SetOutput(stderr)

    projectRoot := flags.String("project", "", "Unity project root")
    connectionFile := flags.String("connection-file", "", "Path to a specific bridge discovery file")
    socketPath := flags.String("socket-path", "", "Direct socket or named pipe path")
    jsonOutput := flags.Bool("json", false, "Print JSON output")

    if err := flags.Parse(args[1:]); err != nil {
        return 1
    }

    currentDir, err := os.Getwd()
    if err != nil {
        fmt.Fprintf(stderr, "failed to resolve current directory: %v\n", err)
        return 1
    }

    resolved, err := discovery.Resolve(discovery.ResolveOptions{
        ProjectRoot:    *projectRoot,
        ConnectionFile: *connectionFile,
        SocketPath:     *socketPath,
        CurrentDir:     currentDir,
    })
    if err != nil {
        fmt.Fprintf(stderr, "failed to resolve Unity bridge: %v\n", err)
        return 1
    }

    payload := map[string]any{
        "projectRoot":    resolved.ProjectRoot,
        "projectAssets":  resolved.ProjectAssets,
        "connectionFile": resolved.ConnectionFile,
        "statusFile":     resolved.StatusFile,
        "connection":     resolved.ConnectionInfo,
        "status":         resolved.StatusInfo,
    }

    if *jsonOutput {
        return printJSON(stdout, payload, false)
    }

    fmt.Fprintf(stdout, "Project Root: %s\n", valueOrUnknown(resolved.ProjectRoot))
    fmt.Fprintf(stdout, "Assets Path: %s\n", valueOrUnknown(resolved.ProjectAssets))
    fmt.Fprintf(stdout, "Connection File: %s\n", valueOrUnknown(resolved.ConnectionFile))
    fmt.Fprintf(stdout, "Connection Path: %s\n", valueOrUnknown(resolved.ConnectionInfo.ConnectionPath))
    fmt.Fprintf(stdout, "Protocol Version: %s\n", valueOrUnknown(resolved.ConnectionInfo.ProtocolVersion))
    if resolved.StatusInfo != nil {
        fmt.Fprintf(stdout, "Heartbeat Status: %s\n", valueOrUnknown(resolved.StatusInfo.Status))
        fmt.Fprintf(stdout, "Last Heartbeat: %s\n", valueOrUnknown(resolved.StatusInfo.LastHeartbeat))
    }

    return 0
}

func runTools(args []string, stdout io.Writer, stderr io.Writer) int {
    if len(args) == 0 {
        fmt.Fprintln(stderr, "tools requires a subcommand: list, describe, or call")
        return 1
    }

    switch args[0] {
    case "list":
        return runToolsList(args[1:], stdout, stderr)
    case "describe":
        return runToolsDescribe(args[1:], stdout, stderr)
    case "call":
        return runToolsCall(args[1:], stdout, stderr)
    default:
        fmt.Fprintf(stderr, "unknown tools subcommand: %s\n", args[0])
        return 1
    }
}

func runToolsList(args []string, stdout io.Writer, stderr io.Writer) int {
    flags := flag.NewFlagSet("tools list", flag.ContinueOnError)
    flags.SetOutput(stderr)

    projectRoot := flags.String("project", "", "Unity project root")
    connectionFile := flags.String("connection-file", "", "Path to a specific bridge discovery file")
    socketPath := flags.String("socket-path", "", "Direct socket or named pipe path")
    jsonOutput := flags.Bool("json", false, "Print JSON output")

    if err := flags.Parse(args); err != nil {
        return 1
    }

    bridgeClient, cleanup, err := openBridgeClient(*projectRoot, *connectionFile, *socketPath)
    if err != nil {
        fmt.Fprintf(stderr, "failed to connect to Unity bridge: %v\n", err)
        return 1
    }
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
    defer cancel()

    tools, err := bridgeClient.ListTools(ctx, "")
    if err != nil {
        fmt.Fprintf(stderr, "failed to list Unity tools: %v\n", err)
        return 1
    }

    if *jsonOutput {
        return printJSON(stdout, tools, false)
    }

    for _, tool := range tools.Tools {
        fmt.Fprintf(stdout, "%s\n", tool.Name)
    }

    return 0
}

func runToolsDescribe(args []string, stdout io.Writer, stderr io.Writer) int {
    toolName, parseArgs, err := peelLeadingToolArg(args)
    if err != nil {
        fmt.Fprintln(stderr, err.Error())
        return 1
    }

    flags := flag.NewFlagSet("tools describe", flag.ContinueOnError)
    flags.SetOutput(stderr)

    projectRoot := flags.String("project", "", "Unity project root")
    connectionFile := flags.String("connection-file", "", "Path to a specific bridge discovery file")
    socketPath := flags.String("socket-path", "", "Direct socket or named pipe path")
    jsonOutput := flags.Bool("json", false, "Print JSON output")

    if err := flags.Parse(parseArgs); err != nil {
        return 1
    }

    if toolName == "" {
        remainingArgs := flags.Args()
        if len(remainingArgs) != 1 {
            fmt.Fprintln(stderr, "tools describe requires a single tool name")
            return 1
        }

        toolName = remainingArgs[0]
    } else if len(flags.Args()) > 0 {
        fmt.Fprintln(stderr, "tools describe accepts only one tool name")
        return 1
    }

    bridgeClient, cleanup, err := openBridgeClient(*projectRoot, *connectionFile, *socketPath)
    if err != nil {
        fmt.Fprintf(stderr, "failed to connect to Unity bridge: %v\n", err)
        return 1
    }
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
    defer cancel()

    tools, err := bridgeClient.ListTools(ctx, "")
    if err != nil {
        fmt.Fprintf(stderr, "failed to list Unity tools: %v\n", err)
        return 1
    }

    for _, tool := range tools.Tools {
        if tool.Name != toolName {
            continue
        }

        if *jsonOutput {
            return printJSON(stdout, tool, false)
        }

        fmt.Fprintf(stdout, "Name: %s\n", tool.Name)
        fmt.Fprintf(stdout, "Title: %s\n", valueOrUnknown(tool.Title))
        fmt.Fprintf(stdout, "Description: %s\n", valueOrUnknown(tool.Description))
        fmt.Fprintln(stdout, "Input Schema:")
        printRawJSON(stdout, tool.InputSchema)
        if len(tool.OutputSchema) > 0 {
            fmt.Fprintln(stdout, "Output Schema:")
            printRawJSON(stdout, tool.OutputSchema)
        }
        if len(tool.Annotations) > 0 {
            fmt.Fprintln(stdout, "Annotations:")
            printRawJSON(stdout, tool.Annotations)
        }
        return 0
    }

    fmt.Fprintf(stderr, "tool not found: %s\n", toolName)
    return 1
}

func runToolsCall(args []string, stdout io.Writer, stderr io.Writer) int {
    toolName, parseArgs, err := peelLeadingToolArg(args)
    if err != nil {
        fmt.Fprintln(stderr, err.Error())
        return 1
    }

    flags := flag.NewFlagSet("tools call", flag.ContinueOnError)
    flags.SetOutput(stderr)

    projectRoot := flags.String("project", "", "Unity project root")
    connectionFile := flags.String("connection-file", "", "Path to a specific bridge discovery file")
    socketPath := flags.String("socket-path", "", "Direct socket or named pipe path")
    jsonArgs := flags.String("json-args", "{}", "JSON encoded tool arguments")

    if err := flags.Parse(parseArgs); err != nil {
        return 1
    }

    if toolName == "" {
        remainingArgs := flags.Args()
        if len(remainingArgs) != 1 {
            fmt.Fprintln(stderr, "tools call requires a single tool name")
            return 1
        }

        toolName = remainingArgs[0]
    } else if len(flags.Args()) > 0 {
        fmt.Fprintln(stderr, "tools call accepts only one tool name")
        return 1
    }

    var toolArguments map[string]any
    if err := json.Unmarshal([]byte(*jsonArgs), &toolArguments); err != nil {
        fmt.Fprintf(stderr, "failed to parse --json-args: %v\n", err)
        return 1
    }

    bridgeClient, cleanup, err := openBridgeClient(*projectRoot, *connectionFile, *socketPath)
    if err != nil {
        fmt.Fprintf(stderr, "failed to connect to Unity bridge: %v\n", err)
        return 1
    }
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := bridgeClient.CallTool(ctx, toolName, toolArguments)
    if err != nil {
        fmt.Fprintf(stderr, "failed to call Unity tool %s: %v\n", toolName, err)
        return 1
    }

    if len(bytes.TrimSpace(result)) == 0 {
        _, _ = fmt.Fprintln(stdout, "{}")
        return 0
    }

    if _, err := stdout.Write(result); err != nil {
        fmt.Fprintf(stderr, "failed to write tool result: %v\n", err)
        return 1
    }
    _, _ = fmt.Fprintln(stdout)
    return 0
}

func runServeMcp(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
    flags := flag.NewFlagSet("serve-mcp", flag.ContinueOnError)
    flags.SetOutput(stderr)

    projectRoot := flags.String("project", "", "Unity project root")
    connectionFile := flags.String("connection-file", "", "Path to a specific bridge discovery file")
    socketPath := flags.String("socket-path", "", "Direct socket or named pipe path")

    if err := flags.Parse(args); err != nil {
        return 1
    }

    server := mcpserver.New(func(ctx context.Context) (unitybridge.Bridge, error) {
        bridgeClient, _, err := openBridgeClient(*projectRoot, *connectionFile, *socketPath)
        if err != nil {
            return nil, err
        }
        return bridgeClient, nil
    })

    if err := server.Serve(context.Background(), stdin, stdout); err != nil && !errors.Is(err, io.EOF) {
        fmt.Fprintf(stderr, "MCP server failed: %v\n", err)
        return 1
    }

    return 0
}

func openBridgeClient(projectRoot string, connectionFile string, socketPath string) (*unitybridge.Client, func(), error) {
    currentDir, err := os.Getwd()
    if err != nil {
        return nil, nil, err
    }

    resolved, err := discovery.Resolve(discovery.ResolveOptions{
        ProjectRoot:    projectRoot,
        ConnectionFile: connectionFile,
        SocketPath:     socketPath,
        CurrentDir:     currentDir,
    })
    if err != nil {
        return nil, nil, err
    }

    ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
    defer cancel()

    bridgeClient, err := unitybridge.Connect(ctx, resolved, clientName)
    if err != nil {
        return nil, nil, err
    }

    cleanup := func() {
        _ = bridgeClient.Close()
    }

    return bridgeClient, cleanup, nil
}

func printJSON(writer io.Writer, payload any, pretty bool) int {
    var data []byte
    var err error
    if pretty {
        data, err = json.MarshalIndent(payload, "", "  ")
    } else {
        data, err = json.Marshal(payload)
    }
    if err != nil {
        return 1
    }

    _, _ = writer.Write(data)
    _, _ = fmt.Fprintln(writer)
    return 0
}

func printRawJSON(writer io.Writer, payload []byte) {
    if len(strings.TrimSpace(string(payload))) == 0 {
        _, _ = fmt.Fprintln(writer, "{}")
        return
    }

    var decoded any
    if err := json.Unmarshal(payload, &decoded); err != nil {
        _, _ = fmt.Fprintf(writer, "%s\n", string(payload))
        return
    }

    formatted, err := json.MarshalIndent(decoded, "", "  ")
    if err != nil {
        _, _ = fmt.Fprintf(writer, "%s\n", string(payload))
        return
    }

    _, _ = writer.Write(formatted)
    _, _ = fmt.Fprintln(writer)
}

func printUsage(writer io.Writer) {
    _, _ = fmt.Fprintln(writer, "Usage: unity-ai-cli <command>")
    _, _ = fmt.Fprintln(writer, "")
    _, _ = fmt.Fprintln(writer, "Commands:")
    _, _ = fmt.Fprintln(writer, "  bridge status")
    _, _ = fmt.Fprintln(writer, "  tools list")
    _, _ = fmt.Fprintln(writer, "  tools describe <tool>")
    _, _ = fmt.Fprintln(writer, "  tools call <tool> --json-args '{}'")
    _, _ = fmt.Fprintln(writer, "  serve-mcp")
}

func valueOrUnknown(value string) string {
    if strings.TrimSpace(value) == "" {
        return "unknown"
    }

    return value
}

func peelLeadingToolArg(args []string) (string, []string, error) {
    if len(args) == 0 {
        return "", args, nil
    }

    if strings.HasPrefix(args[0], "-") {
        return "", args, nil
    }

    toolName := args[0]
    if toolName == "" {
        return "", nil, errors.New("tool name cannot be empty")
    }

    return toolName, args[1:], nil
}
