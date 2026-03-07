package discovery

import (
    "os"
    "path/filepath"
    "testing"
)

func TestFindProjectRoot(t *testing.T) {
    tempDir := t.TempDir()
    projectRoot := filepath.Join(tempDir, "project")
    nestedDir := filepath.Join(projectRoot, "Assets", "Scripts")

    if err := os.MkdirAll(nestedDir, 0o755); err != nil {
        t.Fatalf("failed to create test directories: %v", err)
    }

    root, err := FindProjectRoot(nestedDir)
    if err != nil {
        t.Fatalf("expected project root, got error: %v", err)
    }

    if got, want := root, filepath.Clean(projectRoot); got != want {
        t.Fatalf("expected project root %s, got %s", want, got)
    }
}

func TestResolveMatchesProjectSpecificBridge(t *testing.T) {
    tempDir := t.TempDir()
    projectRoot := filepath.Join(tempDir, "project")
    connectionsDir := filepath.Join(tempDir, "connections")

    if err := os.MkdirAll(filepath.Join(projectRoot, "Assets"), 0o755); err != nil {
        t.Fatalf("failed to create project root: %v", err)
    }

    if err := os.MkdirAll(connectionsDir, 0o755); err != nil {
        t.Fatalf("failed to create connections dir: %v", err)
    }

    connectionPath := filepath.Join(connectionsDir, "bridge-12345678.json")
    statusPath := filepath.Join(connectionsDir, "bridge-status-12345678.json")

    connectionJSON := `{
  "connection_type": "named_pipe",
  "connection_path": "/tmp/unity-mcp-12345678",
  "project_path": "` + filepath.Join(projectRoot, "Assets") + `",
  "protocol_version": "2.0"
}`
    statusJSON := `{
  "connection_type": "named_pipe",
  "connection_path": "/tmp/unity-mcp-12345678",
  "status": "ready",
  "project_path": "` + filepath.Join(projectRoot, "Assets") + `",
  "protocol_version": "2.0"
}`

    if err := os.WriteFile(connectionPath, []byte(connectionJSON), 0o644); err != nil {
        t.Fatalf("failed to write connection file: %v", err)
    }

    if err := os.WriteFile(statusPath, []byte(statusJSON), 0o644); err != nil {
        t.Fatalf("failed to write status file: %v", err)
    }

    resolved, err := Resolve(ResolveOptions{
        ProjectRoot:    projectRoot,
        ConnectionsDir: connectionsDir,
        CurrentDir:     projectRoot,
    })
    if err != nil {
        t.Fatalf("expected resolved bridge, got error: %v", err)
    }

    if resolved.ConnectionInfo.ConnectionPath != "/tmp/unity-mcp-12345678" {
        t.Fatalf("unexpected connection path: %s", resolved.ConnectionInfo.ConnectionPath)
    }

    if resolved.StatusInfo == nil || resolved.StatusInfo.Status != "ready" {
        t.Fatalf("expected ready status info, got %#v", resolved.StatusInfo)
    }
}
