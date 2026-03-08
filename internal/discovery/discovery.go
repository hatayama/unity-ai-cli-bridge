package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	defaultProtocolVersion = "2.0"
)

type ConnectionInfo struct {
	ConnectionType  string `json:"connection_type"`
	ConnectionPath  string `json:"connection_path"`
	CreatedDate     string `json:"created_date"`
	ProjectPath     string `json:"project_path"`
	ProtocolVersion string `json:"protocol_version"`
}

type StatusInfo struct {
	ConnectionType  string `json:"connection_type"`
	ConnectionPath  string `json:"connection_path"`
	Status          string `json:"status"`
	ProjectPath     string `json:"project_path"`
	LastHeartbeat   string `json:"last_heartbeat"`
	ProtocolVersion string `json:"protocol_version"`
}

type ResolveOptions struct {
	ProjectRoot    string
	ConnectionFile string
	SocketPath     string
	ConnectionsDir string
	CurrentDir     string
}

type ResolvedBridge struct {
	ProjectRoot    string
	ProjectAssets  string
	ConnectionFile string
	StatusFile     string
	ConnectionInfo ConnectionInfo
	StatusInfo     *StatusInfo
	ExplicitSocket bool
}

func Resolve(options ResolveOptions) (ResolvedBridge, error) {
	if options.SocketPath != "" {
		resolved := ResolvedBridge{
			ProjectRoot:    cleanOptionalPath(options.ProjectRoot),
			ProjectAssets:  assetsPathForProject(options.ProjectRoot),
			ConnectionFile: "",
			StatusFile:     "",
			ExplicitSocket: true,
			ConnectionInfo: ConnectionInfo{
				ConnectionType:  defaultConnectionType(),
				ConnectionPath:  options.SocketPath,
				ProjectPath:     assetsPathForProject(options.ProjectRoot),
				ProtocolVersion: defaultProtocolVersion,
			},
		}
		return resolved, nil
	}

	connectionsDir, err := resolveConnectionsDir(options.ConnectionsDir)
	if err != nil {
		return ResolvedBridge{}, err
	}

	if options.ConnectionFile != "" {
		return resolveFromConnectionFile(cleanOptionalPath(options.ProjectRoot), options.ConnectionFile)
	}

	projectRoot, err := resolveProjectRoot(options.ProjectRoot, options.CurrentDir)
	if err != nil {
		return ResolvedBridge{}, err
	}

	connectionFiles, err := filepath.Glob(filepath.Join(connectionsDir, "bridge-*.json"))
	if err != nil {
		return ResolvedBridge{}, fmt.Errorf("failed to enumerate bridge files: %w", err)
	}

	sort.Strings(connectionFiles)

	var matches []ResolvedBridge
	targetAssets := cleanOptionalPath(filepath.Join(projectRoot, "Assets"))
	for _, path := range connectionFiles {
		if strings.Contains(filepath.Base(path), "bridge-status-") {
			continue
		}

		resolved, resolveErr := resolveFromConnectionFile(projectRoot, path)
		if resolveErr != nil {
			continue
		}

		if samePath(cleanOptionalPath(resolved.ConnectionInfo.ProjectPath), targetAssets) {
			matches = append(matches, resolved)
		}
	}

	if len(matches) == 0 {
		return ResolvedBridge{}, fmt.Errorf("no Unity MCP bridge found for project %s", projectRoot)
	}

	if len(matches) > 1 {
		return ResolvedBridge{}, fmt.Errorf("multiple Unity MCP bridges matched project %s", projectRoot)
	}

	return matches[0], nil
}

func FindProjectRoot(startDir string) (string, error) {
	if startDir == "" {
		return "", errors.New("current directory is required to discover the Unity project")
	}

	current := cleanOptionalPath(startDir)
	for {
		assetsPath := filepath.Join(current, "Assets")
		info, err := os.Stat(assetsPath)
		if err == nil && info.IsDir() {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("could not find a Unity project from %s", startDir)
}

func resolveConnectionsDir(override string) (string, error) {
	if override != "" {
		return cleanOptionalPath(override), nil
	}

	envDir := strings.TrimSpace(os.Getenv("UNITY_MCP_STATUS_DIR"))
	if envDir != "" {
		return cleanOptionalPath(envDir), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".unity", "mcp", "connections"), nil
}

func resolveProjectRoot(projectRoot string, currentDir string) (string, error) {
	if projectRoot != "" {
		return cleanOptionalPath(projectRoot), nil
	}

	return FindProjectRoot(currentDir)
}

func resolveFromConnectionFile(projectRoot string, connectionFile string) (ResolvedBridge, error) {
	connectionInfo, err := loadConnectionInfo(connectionFile)
	if err != nil {
		return ResolvedBridge{}, err
	}

	statusFile := matchingStatusFile(connectionFile)
	statusInfo, _ := loadStatusInfo(statusFile)

	resolved := ResolvedBridge{
		ProjectRoot:    cleanOptionalPath(projectRoot),
		ProjectAssets:  assetsPathForProject(projectRoot),
		ConnectionFile: cleanOptionalPath(connectionFile),
		StatusFile:     statusFile,
		ConnectionInfo: connectionInfo,
		StatusInfo:     statusInfo,
	}

	if resolved.ProjectRoot == "" && connectionInfo.ProjectPath != "" {
		resolved.ProjectRoot = filepath.Dir(cleanOptionalPath(connectionInfo.ProjectPath))
		resolved.ProjectAssets = cleanOptionalPath(connectionInfo.ProjectPath)
	}

	return resolved, nil
}

func loadConnectionInfo(path string) (ConnectionInfo, error) {
	var info ConnectionInfo

	data, err := os.ReadFile(path)
	if err != nil {
		return info, fmt.Errorf("failed to read connection file %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("failed to parse connection file %s: %w", path, err)
	}

	if info.ConnectionPath == "" {
		return info, fmt.Errorf("connection file %s is missing connection_path", path)
	}

	if info.ProtocolVersion == "" {
		info.ProtocolVersion = defaultProtocolVersion
	}

	return info, nil
}

func loadStatusInfo(path string) (*StatusInfo, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var info StatusInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	if info.ProtocolVersion == "" {
		info.ProtocolVersion = defaultProtocolVersion
	}

	return &info, nil
}

func matchingStatusFile(connectionFile string) string {
	base := filepath.Base(connectionFile)
	if !strings.HasPrefix(base, "bridge-") {
		return ""
	}

	suffix := strings.TrimPrefix(base, "bridge-")
	return filepath.Join(filepath.Dir(connectionFile), "bridge-status-"+suffix)
}

func assetsPathForProject(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}

	return cleanOptionalPath(filepath.Join(projectRoot, "Assets"))
}

func samePath(left string, right string) bool {
	return cleanOptionalPath(left) == cleanOptionalPath(right)
}

func cleanOptionalPath(path string) string {
	if path == "" {
		return ""
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}

	return filepath.Clean(absolutePath)
}

func defaultConnectionType() string {
	if runtime.GOOS == "windows" {
		return "named_pipe"
	}

	return "unix_socket"
}
