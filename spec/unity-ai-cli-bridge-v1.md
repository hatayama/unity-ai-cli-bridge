# Unity AI CLI Bridge V1 Specification

## Goal

Build a Go-based CLI that connects directly to Unity's `UnityMCPBridge` over IPC without using the relay binary.

The CLI must provide:

- direct human-facing commands for bridge status, tool discovery, and tool execution
- an MCP stdio adapter so AI clients can discover and call Unity tools through standard MCP flows
- a minimal Skill document that explains how to use the CLI and MCP adapter safely

## Source Of Truth

The authoritative tool catalog is Unity's direct bridge command:

- `get_available_tools`

Tool discovery must not rely on a static list embedded in the CLI or Skill.

Human discovery path:

- `unity-ai-cli tools list`
- `unity-ai-cli tools describe <tool>`

AI discovery path:

- `unity-ai-cli serve-mcp`
- MCP `tools/list`

## Discovery And Connection

The CLI must resolve the active Unity bridge from discovery files stored under:

- `~/.unity/mcp/connections/bridge-*.json`

Each discovery file contains:

- connection type
- connection path
- project `Assets` path
- protocol version

Resolution rules:

1. If `--socket-path` is provided, use it directly.
2. Else if `--connection-file` is provided, load that file.
3. Else resolve the Unity project root:
   - use `--project` when provided
   - otherwise walk upward from the current directory until a directory containing `Assets` is found
4. Match `<project-root>/Assets` against the discovery file `project_path`
5. If there is no matching bridge, fail with a clear error

The CLI must read the paired heartbeat file when present:

- `bridge-status-*.json`

This file is used by `bridge status` to report readiness without triggering a live client connection.

## Unity Bridge Protocol

The CLI must implement Unity's direct bridge protocol v2.0.

Protocol characteristics:

- server-first handshake
- newline-delimited JSON messages
- command request envelope:
  - `type`
  - `params`
  - `requestId`

The client must handle these bridge message types:

- `handshake`
- `approval_pending`
- `approval_denied`
- `command_in_progress`

The client must support these commands:

- `set_client_info`
- `get_available_tools`
- `<Unity tool name>`

The client must always send a `requestId` so responses can be matched reliably.

## CLI Commands

### `bridge status`

Show:

- project root
- Assets path
- discovery file path
- connection path
- protocol version
- heartbeat status when available

This command should not require a live bridge connection.

### `tools list`

Connect to Unity, set client info, call `get_available_tools`, and print the tool list.

Requirements:

- support `--json`
- preserve Unity tool names exactly
- include schemas in JSON mode

### `tools describe <tool>`

Resolve a single tool from `get_available_tools` and print:

- name
- title
- description
- input schema
- output schema
- annotations

### `tools call <tool> --json-args '<json>'`

Call a Unity tool directly.

Requirements:

- default `params` to `{}` when omitted
- print result JSON on stdout
- print actionable errors on stderr
- return non-zero exit code on failure

## MCP Adapter

`serve-mcp` must expose a standard JSON-RPC over stdio MCP server.

Required methods:

- `initialize`
- `tools/list`
- `tools/call`
- `ping`

Behavior:

- `initialize` returns server info and tool capability support
- `tools/list` maps Unity `get_available_tools` results into MCP tool definitions
- `tools/call` maps MCP call arguments into Unity direct bridge commands
- Unity tool names remain unchanged in V1

Tool call result behavior:

- if Unity returns a successful structured object, expose it through `structuredContent`
- also include a text payload containing JSON for broad client compatibility
- if Unity tool execution fails, return `isError: true`

## Skill

Provide a minimal Skill document under the repository's local skill directory.

The Skill must explain:

- how to start or use `serve-mcp`
- that tool discovery is dynamic
- that the agent should inspect available tools before acting
- common Unity workflows:
  - read console
  - inspect scenes
  - inspect GameObjects
  - inspect assets
  - inspect or edit scripts

The Skill must not hardcode a complete static list of Unity tools.

## Implementation Layout

- `cmd/unity-ai-cli/`
- `internal/cli/`
- `internal/discovery/`
- `internal/transport/`
- `internal/protocol/`
- `internal/unitybridge/`
- `internal/mcpserver/`
- `tests/testdata/`
- `spec/`
- local skill directory

## Test Strategy

Development must proceed in small slices.

For each slice:

1. run Go unit tests
2. run fake bridge integration tests
3. run a real Unity smoke check with `uloop`

The work must not rely only on fake tests until the end.

### Slice sequence

1. discovery
2. transport
3. protocol
4. tool discovery
5. tool execution
6. MCP adapter
7. Skill verification

### Automated coverage

Unit tests:

- discovery resolution
- project matching
- protocol message parsing
- response normalization
- MCP mapping

Integration tests:

- fake bridge handshake
- approval pending flow
- approval denied flow
- command in progress flow
- `get_available_tools`
- direct tool execution
- MCP `tools/list`
- MCP `tools/call`

### Live Unity smoke checks

Use `uloop` for repeated validation during implementation.

Default smoke commands:

- `bridge status`
- `tools list --json`
- `tools describe Unity.ReadConsole`
- `tools call Unity.ReadConsole --json-args '{}'`

At major milestones, run an end-to-end MCP smoke check through `serve-mcp`.

## Acceptance Criteria

- the CLI connects directly to Unity's bridge without relay
- `tools list` returns live Unity tool metadata
- `tools call` can invoke at least `Unity.ReadConsole`
- `serve-mcp` exposes Unity tools through MCP `tools/list` and `tools/call`
- approval waiting and denial are handled correctly
- fake bridge tests and live Unity smoke checks both pass
- the Skill works with dynamic tool discovery
