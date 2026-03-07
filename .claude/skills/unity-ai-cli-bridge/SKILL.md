# Unity AI CLI Bridge

Use this skill when you need to interact with the local Unity Editor through `unity-ai-cli`.

## Workflow

1. Confirm the Unity project is open.
2. Use `unity-ai-cli tools list --json` to discover the currently available tools.
3. Use `unity-ai-cli tools describe <tool>` before calling a tool you have not used yet.
4. Use `unity-ai-cli tools call <tool> --json-args '<json>'` for direct human-driven execution.
5. Use `unity-ai-cli serve-mcp` when an AI client needs standard MCP `tools/list` and `tools/call`.

## Important Rules

- Tool discovery is dynamic. Do not assume a static tool catalog.
- Prefer discovery before execution when the project or package state may have changed.
- Dangerous editor mutations should still be confirmed explicitly in the conversation.
- If Unity reports connection approval pending, accept the client in Project Settings > AI > Unity MCP before retrying.

## Common Starting Points

- Read console output:
  - `unity-ai-cli tools call Unity.ReadConsole --json-args '{}'`
- Inspect available tools:
  - `unity-ai-cli tools list --json`
- Expose Unity to an MCP client:
  - `unity-ai-cli serve-mcp`
