using System.IO;
using UnityEditor;
using UnityEngine;

namespace Hatayama.UnityAiCliBridge.Editor
{
    internal static class BridgeDiagnosticsMenu
    {
        [MenuItem(BridgePackageConstants.MenuRoot + "Refresh Snapshots")]
        public static void RefreshSnapshots()
        {
            BridgeDiagnosticsSnapshotWriter.RefreshSnapshots();
            Debug.Log($"unity-ai-cli diagnostics refreshed at {BridgePackagePaths.SnapshotDirectoryPath}");
        }

        [MenuItem(BridgePackageConstants.MenuRoot + "Open Unity MCP Settings")]
        public static void OpenSettings()
        {
            SettingsService.OpenProjectSettings(BridgePackageConstants.ProjectSettingsPath);
        }

        [MenuItem(BridgePackageConstants.MenuRoot + "Print Diagnostics Summary")]
        public static void PrintDiagnosticsSummary()
        {
            BridgeDiagnosticsSnapshot snapshot = BridgeDiagnosticsService.BuildDiagnosticsSnapshot();
            string diagnosticsPath = BridgePackagePaths.DiagnosticsFilePath;
            string toolPath = BridgePackagePaths.ToolsFilePath;

            Debug.Log(
                "unity-ai-cli diagnostics\n" +
                $"bridgeEnabled: {snapshot.bridgeEnabled}\n" +
                $"bridgeRunning: {snapshot.bridgeRunning}\n" +
                $"activeClientCount: {snapshot.activeClientCount}\n" +
                $"toolCount: {snapshot.toolCount}\n" +
                $"diagnosticsFile: {diagnosticsPath}\n" +
                $"toolsFile: {toolPath}");
        }

        [MenuItem(BridgePackageConstants.MenuRoot + "Emit Sample Console Logs")]
        public static void EmitSampleConsoleLogs()
        {
            BridgeDebugLogEmitter.EmitSampleLogs();
        }

        [MenuItem(BridgePackageConstants.MenuRoot + "Reveal Snapshot Directory")]
        public static void RevealSnapshotDirectory()
        {
            Directory.CreateDirectory(BridgePackagePaths.SnapshotDirectoryPath);
            EditorUtility.RevealInFinder(BridgePackagePaths.SnapshotDirectoryPath);
        }
    }
}
