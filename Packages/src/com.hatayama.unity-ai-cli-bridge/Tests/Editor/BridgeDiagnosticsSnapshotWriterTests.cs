using System.IO;
using NUnit.Framework;
using UnityEngine;

namespace Hatayama.UnityAiCliBridge.Editor.Tests
{
    public class BridgeDiagnosticsSnapshotWriterTests
    {
        [Test]
        public void RefreshSnapshots_WritesDiagnosticsAndToolFiles()
        {
            BridgeDiagnosticsSnapshotWriter.RefreshSnapshots();

            Assert.That(File.Exists(BridgePackagePaths.DiagnosticsFilePath), Is.True);
            Assert.That(File.Exists(BridgePackagePaths.ToolsFilePath), Is.True);

            string diagnosticsJson = File.ReadAllText(BridgePackagePaths.DiagnosticsFilePath);
            string toolsJson = File.ReadAllText(BridgePackagePaths.ToolsFilePath);

            Hatayama.UnityAiCliBridge.BridgeDiagnosticsSnapshot diagnosticsSnapshot =
                JsonUtility.FromJson<Hatayama.UnityAiCliBridge.BridgeDiagnosticsSnapshot>(diagnosticsJson);
            Hatayama.UnityAiCliBridge.ToolSnapshot toolSnapshot =
                JsonUtility.FromJson<Hatayama.UnityAiCliBridge.ToolSnapshot>(toolsJson);

            Assert.That(diagnosticsSnapshot, Is.Not.Null);
            Assert.That(toolSnapshot, Is.Not.Null);
            Assert.That(diagnosticsSnapshot.schemaVersion, Is.EqualTo(BridgePackageConstants.SchemaVersion));
            Assert.That(toolSnapshot.schemaVersion, Is.EqualTo(BridgePackageConstants.SchemaVersion));
        }
    }
}
