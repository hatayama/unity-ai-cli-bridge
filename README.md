# unity-ai-cli-bridge

Experimental direct-bridge tooling for Unity MCP.

This repository contains:

- a Go CLI that connects directly to Unity's built-in `UnityMCPBridge`
- a local Unity companion package under `Packages/src/com.hatayama.unity-ai-cli-bridge`

The Unity-managed `com.unity.ai.assistant` package is treated as immutable.  
This project does not rely on editing `Library/PackageCache`.

## Architecture

```text
unity-ai-cli
  |- status
  |- doctor
  |- tools
  |- describe
  |- call
  |- wait
  `- mcp serve
        |
        v
UnityMCPBridge direct IPC
        |
        v
Unity Editor + com.unity.ai.assistant

Packages/src/com.hatayama.unity-ai-cli-bridge
  `- companion diagnostics snapshots
        |
        v
Library/UnityAiCliBridge/diagnostics.json
Library/UnityAiCliBridge/tools.json
```

The relay binary is not part of this runtime path.

## Requirements

- Unity project with `com.unity.ai.assistant`
- Unity MCP enabled in the Editor
- Go 1.25 or later to build the CLI
- approval for the CLI client in `Project Settings > AI > Unity MCP`

## Companion Package

The repository installs a local Unity package from:

- `Packages/src/com.hatayama.unity-ai-cli-bridge`

The package adds:

- diagnostics snapshots for the CLI
- Editor menu commands for bridge diagnostics
- EditMode tests for the companion layer

The package is wired through `Packages/manifest.json`.

## Build

```sh
go build -o ./bin/unity-ai-cli ./cmd/unity-ai-cli
```

## Usage

Show the currently resolved bridge and companion snapshot state:

```sh
./bin/unity-ai-cli status --json
```

Diagnose discovery, heartbeat, companion snapshots, and likely approval issues:

```sh
./bin/unity-ai-cli doctor --json
```

Wait for bridge readiness:

```sh
./bin/unity-ai-cli wait --for=bridge
```

List available Unity tools from the live direct bridge:

```sh
./bin/unity-ai-cli tools --json
```

Describe one Unity tool:

```sh
./bin/unity-ai-cli describe Unity_GetConsoleLogs --json
```

Call a Unity tool directly:

```sh
./bin/unity-ai-cli call Unity_GetConsoleLogs --json-args '{"maxEntries":5,"includeStackTrace":false}'
```

Expose Unity tools as an MCP stdio server:

```sh
./bin/unity-ai-cli mcp serve
```

## Approval Flow

The first direct connection may stay pending until Unity approves the CLI.

If `doctor`, `tools`, `describe`, `call`, or `mcp serve` reports an approval-related issue:

1. Open `Edit > Project Settings > AI > Unity MCP`
2. Check `Pending Connections`
3. Approve the `unity-ai-cli` client

This approval is handled from the Unity settings UI, not a modal dialog.

## Development

Run Go tests:

```sh
GOCACHE=/tmp/unity-ai-cli-bridge-go-cache go test ./...
```

Compile the Unity project:

```sh
uloop compile
```

Run companion package EditMode tests:

```sh
uloop run-tests --test-mode EditMode --filter-type regex --filter-value "BridgeDiagnosticsServiceTests|BridgeDiagnosticsSnapshotWriterTests"
```

Run the live end-to-end smoke test:

```sh
sh scripts/live-e2e.sh
```

## Specification

- [V1 specification](spec/unity-ai-cli-bridge-v1.md)

## License

[MIT](LICENSE)
