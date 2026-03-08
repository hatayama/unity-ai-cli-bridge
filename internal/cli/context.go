package cli

import (
	"context"
	"flag"
	"io"
	"os"
	"time"

	"github.com/hatayama/unity-ai-cli-bridge/internal/diagnostics"
	"github.com/hatayama/unity-ai-cli-bridge/internal/discovery"
	"github.com/hatayama/unity-ai-cli-bridge/internal/unitybridge"
)

type connectionOptions struct {
	projectRoot    *string
	connectionFile *string
	socketPath     *string
}

type bridgeContext struct {
	Resolved      discovery.ResolvedBridge
	Diagnostics   *diagnostics.DiagnosticsSnapshot
	ToolSnapshot  *diagnostics.ToolSnapshot
	SnapshotPaths diagnostics.SnapshotPaths
}

func bindConnectionFlags(flags *flag.FlagSet) *connectionOptions {
	return &connectionOptions{
		projectRoot:    flags.String("project", "", "Unity project root"),
		connectionFile: flags.String("connection-file", "", "Path to a specific bridge discovery file"),
		socketPath:     flags.String("socket-path", "", "Direct socket or named pipe path"),
	}
}

func (options connectionOptions) resolve() (discovery.ResolvedBridge, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return discovery.ResolvedBridge{}, err
	}

	return discovery.Resolve(discovery.ResolveOptions{
		ProjectRoot:    *options.projectRoot,
		ConnectionFile: *options.connectionFile,
		SocketPath:     *options.socketPath,
		CurrentDir:     currentDir,
	})
}

func (options connectionOptions) resolveProjectRoot() string {
	if options.projectRoot != nil && *options.projectRoot != "" {
		return *options.projectRoot
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	projectRoot, err := discovery.FindProjectRoot(currentDir)
	if err != nil {
		return ""
	}

	return projectRoot
}

func resolveBridgeContext(options *connectionOptions) (bridgeContext, error) {
	resolved, err := options.resolve()
	if err != nil {
		return bridgeContext{}, err
	}

	diagnosticsSnapshot, snapshotPaths, _ := diagnostics.LoadDiagnostics(resolved.ProjectRoot)
	toolSnapshot, _, _ := diagnostics.LoadToolSnapshot(resolved.ProjectRoot)

	return bridgeContext{
		Resolved:      resolved,
		Diagnostics:   diagnosticsSnapshot,
		ToolSnapshot:  toolSnapshot,
		SnapshotPaths: snapshotPaths,
	}, nil
}

func openBridgeClient(options connectionOptions, timeout time.Duration) (*unitybridge.Client, func(), error) {
	resolved, err := options.resolve()
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

func loadToolList(options connectionOptions, timeout time.Duration) (unitybridge.ToolListResult, func(), error) {
	bridgeClient, cleanup, err := openBridgeClient(options, timeout)
	if err != nil {
		return unitybridge.ToolListResult{}, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	toolList, err := bridgeClient.ListTools(ctx, "")
	if err != nil {
		cleanup()
		return unitybridge.ToolListResult{}, nil, err
	}

	return toolList, cleanup, nil
}

type loadedToolList struct {
	ToolList unitybridge.ToolListResult
	Cleanup  func()
}

func loadToolListWithSpinner(
	stderr io.Writer,
	options connectionOptions,
	timeout time.Duration,
) (unitybridge.ToolListResult, func(), error) {
	result, err := runWithSpinner(
		stderr,
		"Loading enabled Unity tools from Unity",
		func() (loadedToolList, error) {
			toolList, cleanup, loadErr := loadToolList(options, timeout)
			if loadErr != nil {
				return loadedToolList{}, loadErr
			}

			return loadedToolList{
				ToolList: toolList,
				Cleanup:  cleanup,
			}, nil
		},
	)
	if err != nil {
		return unitybridge.ToolListResult{}, nil, err
	}

	return result.ToolList, result.Cleanup, nil
}
