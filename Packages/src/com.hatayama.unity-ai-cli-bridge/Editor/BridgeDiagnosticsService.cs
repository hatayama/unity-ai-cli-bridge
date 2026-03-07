using System;
using System.Diagnostics;
using System.Linq;
using Hatayama.UnityAiCliBridge;
using Unity.AI.MCP.Editor;
using Unity.AI.MCP.Editor.ToolRegistry;
using UnityEditor.PackageManager;

namespace Hatayama.UnityAiCliBridge.Editor
{
    public static class BridgeDiagnosticsService
    {
        public static BridgeDiagnosticsSnapshot BuildDiagnosticsSnapshot()
        {
            McpToolInfo[] tools = McpToolRegistry.GetAvailableTools();
            string[] activeIdentityKeys = UnityMCPBridge.GetActiveIdentityKeys();

            BridgeDiagnosticsSnapshot snapshot = new BridgeDiagnosticsSnapshot
            {
                schemaVersion = BridgePackageConstants.SchemaVersion,
                projectPath = BridgePackagePaths.ProjectRootPath,
                unityVersion = UnityEngine.Application.unityVersion,
                packageName = BridgePackageConstants.PackageName,
                packageVersion = ResolvePackageVersion(),
                settingsPath = BridgePackageConstants.ProjectSettingsPath,
                bridgeEnabled = UnityMCPBridge.Enabled,
                bridgeRunning = UnityMCPBridge.IsRunning,
                activeClientCount = UnityMCPBridge.GetConnectedClientCount(),
                activeIdentityKeys = activeIdentityKeys,
                toolCount = tools.Length,
                toolNames = tools.Select(toolInfo => toolInfo.name).ToArray(),
                generatedAtUtc = DateTime.UtcNow.ToString("O"),
            };

            Debug.Assert(snapshot.activeIdentityKeys != null, "Active identity keys must be present");
            Debug.Assert(snapshot.toolNames != null, "Tool names must be present");
            return snapshot;
        }

        public static ToolSnapshot BuildToolSnapshot()
        {
            McpToolInfo[] tools = McpToolRegistry.GetAvailableTools();
            ToolSnapshotEntry[] entries = tools.Select(BuildToolSnapshotEntry).ToArray();

            ToolSnapshot snapshot = new ToolSnapshot
            {
                schemaVersion = BridgePackageConstants.SchemaVersion,
                generatedAtUtc = DateTime.UtcNow.ToString("O"),
                tools = entries,
            };

            Debug.Assert(snapshot.tools != null, "Tool snapshot entries must be present");
            return snapshot;
        }

        static ToolSnapshotEntry BuildToolSnapshotEntry(McpToolInfo toolInfo)
        {
            Debug.Assert(toolInfo != null, "Tool info must not be null");

            return new ToolSnapshotEntry
            {
                name = toolInfo.name,
                title = toolInfo.title,
                description = toolInfo.description,
            };
        }

        static string ResolvePackageVersion()
        {
            PackageInfo packageInfo = PackageInfo.FindForAssembly(typeof(BridgeDiagnosticsService).Assembly);
            if (packageInfo == null)
            {
                return string.Empty;
            }

            return packageInfo.version;
        }
    }
}
