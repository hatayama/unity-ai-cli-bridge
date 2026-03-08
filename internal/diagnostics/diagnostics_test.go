package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsForProject(t *testing.T) {
	projectRoot := filepath.Join("/tmp", "project")

	paths := PathsForProject(projectRoot)

	expectedDirectory := filepath.Join(projectRoot, "Library", directoryName)
	if paths.Directory != expectedDirectory {
		t.Fatalf("expected diagnostics directory %s, got %s", expectedDirectory, paths.Directory)
	}

	expectedDiagnosticsFile := filepath.Join(expectedDirectory, diagnosticsFileName)
	if paths.DiagnosticsFile != expectedDiagnosticsFile {
		t.Fatalf("expected diagnostics file %s, got %s", expectedDiagnosticsFile, paths.DiagnosticsFile)
	}

	expectedToolsFile := filepath.Join(expectedDirectory, toolsFileName)
	if paths.ToolsFile != expectedToolsFile {
		t.Fatalf("expected tools file %s, got %s", expectedToolsFile, paths.ToolsFile)
	}
}

func TestLoadDiagnostics(t *testing.T) {
	projectRoot := t.TempDir()
	paths := PathsForProject(projectRoot)

	if err := os.MkdirAll(paths.Directory, 0o755); err != nil {
		t.Fatalf("failed to create diagnostics directory: %v", err)
	}

	payload := `{
  "schemaVersion": 1,
  "projectPath": "/tmp/project",
  "bridgeRunning": true,
  "toolCount": 7,
  "generatedAtUtc": "2026-03-07T00:00:00Z"
}`
	if err := os.WriteFile(paths.DiagnosticsFile, []byte(payload), 0o644); err != nil {
		t.Fatalf("failed to write diagnostics snapshot: %v", err)
	}

	snapshot, _, err := LoadDiagnostics(projectRoot)
	if err != nil {
		t.Fatalf("expected diagnostics snapshot to load, got error: %v", err)
	}

	if !snapshot.BridgeRunning {
		t.Fatal("expected bridgeRunning to be true")
	}

	if snapshot.ToolCount != 7 {
		t.Fatalf("expected toolCount 7, got %d", snapshot.ToolCount)
	}
}
