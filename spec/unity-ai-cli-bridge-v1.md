# Unity AI CLI Bridge V1 Specification

## Summary

V1 consists of two tracked deliverables:

- a standalone Go CLI that talks directly to Unity's built-in direct MCP bridge
- a local Unity companion package under `Packages/src/com.hatayama.unity-ai-cli-bridge`

`com.unity.ai.assistant` is treated as an immutable external dependency.  
V1 must not depend on editing `Library/PackageCache` or any Unity-managed package internals.

## Architecture

### CLI

The CLI remains the primary runtime surface.

Required commands:

- `unity-ai-cli status`
- `unity-ai-cli doctor`
- `unity-ai-cli help [tool]`
- `unity-ai-cli recipes [recipe-id]`
- `unity-ai-cli tools`
- `unity-ai-cli describe <tool>`
- `unity-ai-cli call <tool> --json-args '<json>'`
- `unity-ai-cli wait`
- `unity-ai-cli mcp serve`

The CLI must continue to resolve Unity direct bridge connection files from:

- `~/.unity/mcp/connections/bridge-*.json`
- paired heartbeat files `bridge-status-*.json`

The CLI must talk directly to the Unity bridge over:

- Unix sockets on macOS/Linux
- Named pipes on Windows

### Unity companion package

The companion package lives at:

- `Packages/src/com.hatayama.unity-ai-cli-bridge`

Its job is diagnostics and integration support only. It must not patch or override Unity MCP internals.

The package writes companion snapshots to:

- `Library/UnityAiCliBridge/diagnostics.json`
- `Library/UnityAiCliBridge/tools.json`

The package uses only public Unity MCP APIs such as:

- `UnityMCPBridge`
- `McpToolRegistry.GetAvailableTools()`

## CLI Behavior

### `status`

Show discovery and heartbeat information without requiring a live bridge connection.

If a companion diagnostics snapshot exists, include:

- bridge enabled/running state
- active client count
- tool count
- last diagnostics update

### `doctor`

Diagnose the current project using this precedence:

1. discovery file and heartbeat file
2. companion diagnostics snapshots
3. live bridge probe only when earlier layers do not already show a healthy bridge

The output must classify issues into actionable checks and advice.

Expected advice categories:

- Unity project not found
- Unity bridge not discovered
- heartbeat not ready
- pending approval likely
- direct connection already occupied

### `tools`

Connect live to Unity and call `get_available_tools`.

Requirements:

- support `--json`
- print the exact Unity tool names
- support `--category`
- do not rely on the companion tool snapshot for primary tool execution
- plain text output should include category and short summary when local catalog metadata exists

### `help`

Provide human-oriented help for the currently enabled Unity tools.

Requirements:

- `help` without arguments lists only the tools returned by the live Unity direct bridge
- `help <tool>` succeeds only when the tool is currently enabled and exposed
- support `--json`
- merge live schema data with repo-local help metadata when available
- fall back to live description and schema when no local metadata exists

### `recipes`

Provide task-oriented recipes for the currently enabled Unity tools.

Requirements:

- `recipes` lists only recipes whose required tools are all currently enabled
- `recipes <recipe-id>` fails when the recipe is unknown or currently unavailable
- support `--json`
- use repo-local recipe metadata and live tool availability checks

### `describe`

Resolve a single tool from the live tool list and print:

- name
- title
- description
- input schema
- output schema
- annotations
- a plain-text hint pointing to `unity-ai-cli help <tool>`

### `call`

Execute one Unity tool directly.

Requirements:

- accept `--json-args`
- default to `{}` when omitted
- print raw JSON result on stdout
- use non-zero exit on error

### `wait`

Wait for one of these targets:

- `bridge`
- `status`
- `tools`

Rules:

- `status` waits for a ready heartbeat file
- `bridge` accepts either a ready heartbeat or a running companion diagnostics snapshot
- `tools` uses the companion tool snapshot by default
- `tools --live` waits until a live `get_available_tools` succeeds

### `mcp serve`

Expose a standard stdio MCP server supporting:

- `initialize`
- `ping`
- `tools/list`
- `tools/call`

Tool discovery and tool execution still come from the live Unity direct bridge.

## Companion Package Behavior

### Package contents

The package must include:

- `package.json`
- `Runtime/`
- `Editor/`
- `Tests/Editor/`
- `README.md`
- `CHANGELOG.md`

### Editor features

The package must provide:

- menu command to refresh snapshots
- menu command to open `Project Settings > AI > Unity MCP`
- menu command to print a diagnostics summary
- menu command to reveal the snapshot directory

### Snapshot schema

`diagnostics.json` must contain:

- schema version
- project path
- Unity version
- package name and version
- MCP settings path
- bridge enabled state
- bridge running state
- active client count
- active identity keys
- tool count
- tool names
- generation timestamp

`tools.json` must contain:

- schema version
- generation timestamp
- tools array
- each tool entry includes name, title, and description

### Refresh timing

The companion package refreshes snapshots:

- on editor load
- after assembly reload
- when triggered manually from the menu

V1 does not require continuous polling.

## Tests

### Go

- catalog metadata loading and merge tests
- discovery resolution
- diagnostics snapshot parsing
- direct bridge protocol tests
- MCP server tests
- live CLI E2E covering `status`, `doctor`, `wait`, `help`, `recipes`, `tools`, `describe`, `call`, and `mcp serve`

### Unity

- EditMode tests for diagnostics snapshot generation
- EditMode tests for snapshot writer output
- package compile verification via `uloop compile`

## Acceptance Criteria

- the CLI works without modifying `com.unity.ai.assistant`
- the local companion package is installed through `Packages/manifest.json`
- `status` reports discovery and companion diagnostics when available
- `doctor` provides actionable output for approval and readiness issues
- `help` lists only currently enabled tools and explains one tool in human-oriented terms
- `recipes` lists only currently available workflows based on the enabled tool set
- `tools`, `describe`, and `call` execute through the live Unity direct bridge
- `wait` supports heartbeat-based and snapshot-based readiness checks
- `mcp serve` exposes Unity tools through MCP stdio
- Unity companion package writes both `diagnostics.json` and `tools.json`
