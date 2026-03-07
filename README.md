# unity-ai-cli-bridge

Experimental Go CLI for calling Unity's built-in MCP tools over the direct Unity bridge.

## Overview

Unity's `com.unity.ai.assistant` package already exposes MCP tools inside the Editor. This repository adds a CLI that connects straight to `UnityMCPBridge` over IPC and does not connect to the Unity relay binary.

Current goals:

- discover Unity tools from the command line
- call Unity tools directly with JSON arguments
- expose the same tool surface as an MCP stdio server for AI clients

## Architecture

```text
unity-ai-cli
  |- bridge status
  |- tools list
  |- tools describe
  |- tools call
  `- serve-mcp
        |
        v
UnityMCPBridge direct IPC
  |- Unix socket on macOS/Linux
  `- Named pipe on Windows
        |
        v
Unity Editor
        |
        v
McpToolRegistry

The relay binary is not part of this runtime path.
```

## Status

Implemented and smoke-tested:

- Unity bridge discovery from `~/.unity/mcp/connections/bridge-*.json`
- direct bridge handshake and command execution
- `tools list`, `tools describe`, and `tools call`
- `serve-mcp` for MCP `initialize`, `tools/list`, and `tools/call`

The implementation is still early, but it is already usable for direct local experiments.

## Requirements

- Unity project with `com.unity.ai.assistant`
- Unity MCP enabled in the Editor
- Go 1.25 or later to build from source
- Approval for the CLI client in `Project Settings > AI > Unity MCP`

## Build

```sh
go build -o ./bin/unity-ai-cli ./cmd/unity-ai-cli
```

## Usage

Show the currently resolved bridge:

```sh
./bin/unity-ai-cli bridge status --json
```

List available Unity tools:

```sh
./bin/unity-ai-cli tools list --json
```

Describe one tool:

```sh
./bin/unity-ai-cli tools describe Unity_GetConsoleLogs
```

Call a tool directly:

```sh
./bin/unity-ai-cli tools call Unity_GetConsoleLogs --json-args '{"maxEntries":5,"includeStackTrace":false}'
```

Expose Unity tools as an MCP stdio server:

```sh
./bin/unity-ai-cli serve-mcp
```

## Approval Flow

The first direct connection may remain pending until Unity approves it.

If a command blocks or fails with an approval-related message:

1. Open `Edit > Project Settings > AI > Unity MCP`
2. Find the entry under `Pending Connections`
3. Accept the `unity-ai-cli` client

This approval is handled in the Unity settings UI, not by a modal dialog.

## Notes

- Unity currently exposes tool names in sanitized form such as `Unity_GetConsoleLogs`.
- In the current Unity configuration, the direct bridge may allow only one active direct connection at a time.
- Live validation has been done on macOS. Windows named pipe support is implemented, but not yet live-smoke-tested in this repository.

## Development

Run tests:

```sh
go test ./...
```

Useful local smoke checks:

```sh
./bin/unity-ai-cli bridge status --json
./bin/unity-ai-cli tools list --json
./bin/unity-ai-cli tools describe Unity_GetConsoleLogs
./bin/unity-ai-cli tools call Unity_GetConsoleLogs --json-args '{"maxEntries":5,"includeStackTrace":false}'
```

## Specification

- [V1 specification](spec/unity-ai-cli-bridge-v1.md)

## License

[MIT](LICENSE)
