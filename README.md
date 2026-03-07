# unity-ai-cli-bridge

> **Warning**
> This project is still in early development and is not yet usable. Stay tuned!

A CLI bridge to call Unity's official MCP tools from the command line.

## Overview

Unity 6 introduced an official MCP (Model Context Protocol) integration via the [`com.unity.ai.assistant`](https://docs.unity3d.com/Packages/com.unity.ai.assistant@2.0/manual/unity-mcp-overview.html) package. This enables AI agents to interact with the Unity Editor — managing scenes, assets, scripts, and more.

However, the Unity MCP is currently only accessible through MCP clients such as Claude Code, Cursor, or Windsurf. There is no way to invoke these tools directly from the command line.

**unity-ai-cli-bridge** fills this gap by acting as an MCP client that connects to Unity's relay binary and exposes its tools as CLI commands.

## How Unity MCP Works

```
CLI (this tool)
  ↓ stdio (MCP protocol)
Relay Binary (~/.unity/relay/)
  ↓ IPC (named pipes / unix sockets)
Unity Editor (MCP Bridge)
  ↓
McpToolRegistry (built-in & custom tools)
```

1. Unity Editor runs an internal MCP Bridge
2. A relay binary (installed at `~/.unity/relay/`) acts as an MCP server over stdio
3. This CLI connects to the relay binary as an MCP client
4. Tools registered in Unity (scene management, asset operations, etc.) become available as CLI commands

## Status

🚧 Under development

## License

[MIT](LICENSE)
