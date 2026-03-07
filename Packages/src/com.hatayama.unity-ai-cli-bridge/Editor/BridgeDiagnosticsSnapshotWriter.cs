using System.IO;
using System.Text;
using Hatayama.UnityAiCliBridge;

namespace Hatayama.UnityAiCliBridge.Editor
{
    public static class BridgeDiagnosticsSnapshotWriter
    {
        static readonly UTF8Encoding k_Utf8WithoutBom = new UTF8Encoding(false);

        public static void RefreshSnapshots()
        {
            BridgeDiagnosticsSnapshot diagnosticsSnapshot = BridgeDiagnosticsService.BuildDiagnosticsSnapshot();
            ToolSnapshot toolSnapshot = BridgeDiagnosticsService.BuildToolSnapshot();

            WriteJson(BridgePackagePaths.DiagnosticsFilePath, diagnosticsSnapshot);
            WriteJson(BridgePackagePaths.ToolsFilePath, toolSnapshot);
        }

        static void WriteJson<T>(string path, T payload)
        {
            System.Diagnostics.Debug.Assert(!string.IsNullOrWhiteSpace(path), "Snapshot path must not be empty");
            System.Diagnostics.Debug.Assert(payload != null, "Snapshot payload must not be null");

            string directoryPath = Path.GetDirectoryName(path);
            System.Diagnostics.Debug.Assert(!string.IsNullOrWhiteSpace(directoryPath), "Snapshot directory path must not be empty");

            Directory.CreateDirectory(directoryPath);

            string json = UnityEngine.JsonUtility.ToJson(payload, true);
            File.WriteAllText(path, json, k_Utf8WithoutBom);
        }
    }
}
