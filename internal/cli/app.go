package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

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
	case "status":
		return runStatus(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "help":
		return runHelp(args[1:], stdout, stderr)
	case "recipes":
		return runRecipes(args[1:], stdout, stderr)
	case "tools":
		return runTools(args[1:], stdout, stderr)
	case "describe":
		return runDescribe(args[1:], stdout, stderr)
	case "call":
		return runCall(args[1:], stdout, stderr)
	case "wait":
		return runWait(args[1:], stdout, stderr)
	case "mcp":
		return runMcp(args[1:], stdin, stdout, stderr)
	case "serve-mcp":
		return runMcp(append([]string{"serve"}, args[1:]...), stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonOutput := flags.Bool("json", false, "Print JSON output")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	bridgeContext, err := resolveBridgeContext(options)
	if err != nil {
		fmt.Fprintf(stderr, "failed to resolve Unity bridge: %v\n", err)
		return 1
	}

	payload := map[string]any{
		"projectRoot":    bridgeContext.Resolved.ProjectRoot,
		"projectAssets":  bridgeContext.Resolved.ProjectAssets,
		"connectionFile": bridgeContext.Resolved.ConnectionFile,
		"statusFile":     bridgeContext.Resolved.StatusFile,
		"connection":     bridgeContext.Resolved.ConnectionInfo,
		"status":         bridgeContext.Resolved.StatusInfo,
		"companion": map[string]any{
			"directory":       bridgeContext.SnapshotPaths.Directory,
			"diagnosticsFile": bridgeContext.SnapshotPaths.DiagnosticsFile,
			"toolsFile":       bridgeContext.SnapshotPaths.ToolsFile,
			"diagnostics":     bridgeContext.Diagnostics,
			"tools":           bridgeContext.ToolSnapshot,
		},
	}

	if *jsonOutput {
		return printJSON(stdout, payload, true)
	}

	fmt.Fprintf(stdout, "Project Root: %s\n", valueOrUnknown(bridgeContext.Resolved.ProjectRoot))
	fmt.Fprintf(stdout, "Assets Path: %s\n", valueOrUnknown(bridgeContext.Resolved.ProjectAssets))
	fmt.Fprintf(stdout, "Connection File: %s\n", valueOrUnknown(bridgeContext.Resolved.ConnectionFile))
	fmt.Fprintf(stdout, "Connection Path: %s\n", valueOrUnknown(bridgeContext.Resolved.ConnectionInfo.ConnectionPath))
	fmt.Fprintf(stdout, "Protocol Version: %s\n", valueOrUnknown(bridgeContext.Resolved.ConnectionInfo.ProtocolVersion))
	if bridgeContext.Resolved.StatusInfo != nil {
		fmt.Fprintf(stdout, "Heartbeat Status: %s\n", valueOrUnknown(bridgeContext.Resolved.StatusInfo.Status))
		fmt.Fprintf(stdout, "Last Heartbeat: %s\n", valueOrUnknown(bridgeContext.Resolved.StatusInfo.LastHeartbeat))
	}
	fmt.Fprintf(stdout, "Diagnostics File: %s\n", valueOrUnknown(bridgeContext.SnapshotPaths.DiagnosticsFile))
	if bridgeContext.Diagnostics != nil {
		fmt.Fprintf(stdout, "Bridge Running: %t\n", bridgeContext.Diagnostics.BridgeRunning)
		fmt.Fprintf(stdout, "Active Clients: %d\n", bridgeContext.Diagnostics.ActiveClientCount)
		fmt.Fprintf(stdout, "Tool Count: %d\n", bridgeContext.Diagnostics.ToolCount)
		fmt.Fprintf(stdout, "Diagnostics Updated: %s\n", valueOrUnknown(bridgeContext.Diagnostics.GeneratedAtUTC))
	}

	return 0
}

func runDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonOutput := flags.Bool("json", false, "Print JSON output")
	liveTimeout := flags.Duration("live-timeout", 5*time.Second, "Timeout for an optional live probe")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	report := buildDoctorReport(options, *liveTimeout)
	if *jsonOutput {
		return printJSON(stdout, report, true)
	}

	fmt.Fprintf(stdout, "Project Root: %s\n", valueOrUnknown(report.ProjectRoot))
	fmt.Fprintf(stdout, "Connection File: %s\n", valueOrUnknown(report.ConnectionFile))
	fmt.Fprintf(stdout, "Status File: %s\n", valueOrUnknown(report.StatusFile))
	fmt.Fprintf(stdout, "Diagnostics File: %s\n", valueOrUnknown(report.DiagnosticsFile))
	fmt.Fprintf(stdout, "Tools File: %s\n", valueOrUnknown(report.ToolsFile))
	fmt.Fprintln(stdout, "Checks:")
	for _, check := range report.Checks {
		fmt.Fprintf(stdout, "- [%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
	}
	if len(report.Advice) > 0 {
		fmt.Fprintln(stdout, "Advice:")
		for _, advice := range report.Advice {
			fmt.Fprintf(stdout, "- %s\n", advice)
		}
	}

	if report.HasFailure() {
		return 1
	}

	return 0
}

func runTools(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("tools", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonOutput := flags.Bool("json", false, "Print JSON output")
	category := flags.String("category", "", "Filter tools by catalog category")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	toolList, cleanup, err := loadToolList(*options, 20*time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "failed to connect to Unity bridge: %v\n", err)
		return 1
	}
	defer cleanup()

	if *jsonOutput {
		if strings.TrimSpace(*category) == "" {
			return printJSON(stdout, toolList, true)
		}

		summaries, err := buildToolSummaries(toolList, *category)
		if err != nil {
			fmt.Fprintf(stderr, "failed to build tool summaries: %v\n", err)
			return 1
		}

		allowedToolNames := make(map[string]struct{}, len(summaries))
		for _, summary := range summaries {
			allowedToolNames[summary.Name] = struct{}{}
		}

		filteredTools := make([]unitybridge.ToolInfo, 0, len(summaries))
		for _, tool := range toolList.Tools {
			if _, ok := allowedToolNames[tool.Name]; !ok {
				continue
			}

			filteredTools = append(filteredTools, tool)
		}

		filteredToolList := toolList
		filteredToolList.Tools = filteredTools
		return printJSON(stdout, filteredToolList, true)
	}

	summaries, err := buildToolSummaries(toolList, *category)
	if err != nil {
		fmt.Fprintf(stderr, "failed to build tool summaries: %v\n", err)
		return 1
	}

	printToolSummaries(stdout, summaries)
	return 0
}

func runDescribe(args []string, stdout io.Writer, stderr io.Writer) int {
	toolName, parseArgs, err := peelLeadingToolArg(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	flags := flag.NewFlagSet("describe", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonOutput := flags.Bool("json", false, "Print JSON output")

	if err := flags.Parse(parseArgs); err != nil {
		return 1
	}

	if toolName == "" {
		if len(flags.Args()) != 1 {
			fmt.Fprintln(stderr, "describe requires a single tool name")
			return 1
		}
		toolName = flags.Args()[0]
	} else if len(flags.Args()) > 0 {
		fmt.Fprintln(stderr, "describe accepts only one tool name")
		return 1
	}

	toolList, cleanup, err := loadToolList(*options, 20*time.Second)
	if err != nil {
		fmt.Fprintf(stderr, "failed to connect to Unity bridge: %v\n", err)
		return 1
	}
	defer cleanup()

	for _, tool := range toolList.Tools {
		if tool.Name != toolName {
			continue
		}

		if *jsonOutput {
			return printJSON(stdout, tool, true)
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
		fmt.Fprintf(stdout, "See: unity-ai-cli help %s\n", tool.Name)
		return 0
	}

	fmt.Fprintf(stderr, "tool not found: %s\n", toolName)
	return 1
}

func runCall(args []string, stdout io.Writer, stderr io.Writer) int {
	toolName, parseArgs, err := peelLeadingToolArg(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	flags := flag.NewFlagSet("call", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	jsonArgs := flags.String("json-args", "{}", "JSON encoded tool arguments")

	if err := flags.Parse(parseArgs); err != nil {
		return 1
	}

	if toolName == "" {
		if len(flags.Args()) != 1 {
			fmt.Fprintln(stderr, "call requires a single tool name")
			return 1
		}
		toolName = flags.Args()[0]
	} else if len(flags.Args()) > 0 {
		fmt.Fprintln(stderr, "call accepts only one tool name")
		return 1
	}

	toolArguments := make(map[string]any)
	if err := json.Unmarshal([]byte(*jsonArgs), &toolArguments); err != nil {
		fmt.Fprintf(stderr, "failed to parse --json-args: %v\n", err)
		return 1
	}

	bridgeClient, cleanup, err := openBridgeClient(*options, 30*time.Second)
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

	return printJSONBytes(stdout, result)
}

func runWait(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("wait", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)
	waitFor := flags.String("for", "bridge", "Wait target: bridge, status, or tools")
	timeout := flags.Duration("timeout", 30*time.Second, "Maximum wait duration")
	interval := flags.Duration("interval", 1*time.Second, "Polling interval")
	live := flags.Bool("live", false, "Require a live bridge probe for tools")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	deadline := time.Now().Add(*timeout)
	lastMessage := "not ready yet"
	for {
		ready, message := evaluateWaitTarget(*options, *waitFor, *live)
		if ready {
			fmt.Fprintf(stdout, "%s\n", message)
			return 0
		}

		lastMessage = message
		if time.Now().After(deadline) {
			fmt.Fprintf(stderr, "wait timed out: %s\n", lastMessage)
			return 1
		}

		time.Sleep(*interval)
	}
}

func runMcp(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(stderr, "mcp supports only the serve subcommand")
		return 1
	}

	flags := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	flags.SetOutput(stderr)

	options := bindConnectionFlags(flags)

	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}

	server := mcpserver.New(func(ctx context.Context) (unitybridge.Bridge, error) {
		bridgeClient, _, err := openBridgeClient(*options, 20*time.Second)
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

func printJSONBytes(writer io.Writer, payload []byte) int {
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		_, _ = writer.Write(payload)
		_, _ = fmt.Fprintln(writer)
		return 0
	}

	return printJSON(writer, decoded, true)
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
	_, _ = fmt.Fprintln(writer, "  status")
	_, _ = fmt.Fprintln(writer, "  doctor")
	_, _ = fmt.Fprintln(writer, "  help [tool]")
	_, _ = fmt.Fprintln(writer, "  recipes [recipe-id]")
	_, _ = fmt.Fprintln(writer, "  tools")
	_, _ = fmt.Fprintln(writer, "  describe <tool>")
	_, _ = fmt.Fprintln(writer, "  call <tool> --json-args '{}'")
	_, _ = fmt.Fprintln(writer, "  wait")
	_, _ = fmt.Fprintln(writer, "  mcp serve")
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
