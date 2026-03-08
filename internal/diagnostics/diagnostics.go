package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	directoryName       = "UnityAiCliBridge"
	diagnosticsFileName = "diagnostics.json"
	toolsFileName       = "tools.json"
)

type SnapshotPaths struct {
	Directory       string
	DiagnosticsFile string
	ToolsFile       string
}

type DiagnosticsSnapshot struct {
	SchemaVersion      int      `json:"schemaVersion"`
	ProjectPath        string   `json:"projectPath"`
	UnityVersion       string   `json:"unityVersion"`
	PackageName        string   `json:"packageName"`
	PackageVersion     string   `json:"packageVersion"`
	SettingsPath       string   `json:"settingsPath"`
	BridgeEnabled      bool     `json:"bridgeEnabled"`
	BridgeRunning      bool     `json:"bridgeRunning"`
	ActiveClientCount  int      `json:"activeClientCount"`
	ActiveIdentityKeys []string `json:"activeIdentityKeys"`
	ToolCount          int      `json:"toolCount"`
	ToolNames          []string `json:"toolNames"`
	GeneratedAtUTC     string   `json:"generatedAtUtc"`
}

type ToolSnapshot struct {
	SchemaVersion  int                `json:"schemaVersion"`
	GeneratedAtUTC string             `json:"generatedAtUtc"`
	Tools          []ToolSnapshotInfo `json:"tools"`
}

type ToolSnapshotInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func PathsForProject(projectRoot string) SnapshotPaths {
	cleanProjectRoot := filepath.Clean(strings.TrimSpace(projectRoot))
	libraryDir := filepath.Join(cleanProjectRoot, "Library", directoryName)
	return SnapshotPaths{
		Directory:       libraryDir,
		DiagnosticsFile: filepath.Join(libraryDir, diagnosticsFileName),
		ToolsFile:       filepath.Join(libraryDir, toolsFileName),
	}
}

func LoadDiagnostics(projectRoot string) (*DiagnosticsSnapshot, SnapshotPaths, error) {
	paths := PathsForProject(projectRoot)
	snapshot, err := loadJSON[DiagnosticsSnapshot](paths.DiagnosticsFile)
	if err != nil {
		return nil, paths, err
	}

	return snapshot, paths, nil
}

func LoadToolSnapshot(projectRoot string) (*ToolSnapshot, SnapshotPaths, error) {
	paths := PathsForProject(projectRoot)
	snapshot, err := loadJSON[ToolSnapshot](paths.ToolsFile)
	if err != nil {
		return nil, paths, err
	}

	return snapshot, paths, nil
}

func loadJSON[T any](path string) (*T, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("snapshot path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var snapshot T
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", path, err)
	}

	return &snapshot, nil
}
