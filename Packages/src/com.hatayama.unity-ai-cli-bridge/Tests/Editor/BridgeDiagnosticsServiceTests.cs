using NUnit.Framework;
using UnityEditor;

namespace Hatayama.UnityAiCliBridge.Editor.Tests
{
    public class BridgeDiagnosticsServiceTests
    {
        [Test]
        public void BuildDiagnosticsSnapshot_ReturnsCurrentBridgeState()
        {
            Hatayama.UnityAiCliBridge.BridgeDiagnosticsSnapshot snapshot = BridgeDiagnosticsService.BuildDiagnosticsSnapshot();

            Assert.That(snapshot.schemaVersion, Is.EqualTo(BridgePackageConstants.SchemaVersion));
            Assert.That(snapshot.projectPath, Is.Not.Empty);
            Assert.That(snapshot.packageName, Is.EqualTo(BridgePackageConstants.PackageName));
            Assert.That(snapshot.settingsPath, Is.EqualTo(BridgePackageConstants.ProjectSettingsPath));
            Assert.That(snapshot.activeIdentityKeys, Is.Not.Null);
            Assert.That(snapshot.toolNames, Is.Not.Null);
            Assert.That(snapshot.toolCount, Is.EqualTo(snapshot.toolNames.Length));
            Assert.That(snapshot.generatedAtUtc, Is.Not.Empty);
        }

        [Test]
        public void BuildToolSnapshot_ReturnsToolEntries()
        {
            Hatayama.UnityAiCliBridge.ToolSnapshot snapshot = BridgeDiagnosticsService.BuildToolSnapshot();

            Assert.That(snapshot.schemaVersion, Is.EqualTo(BridgePackageConstants.SchemaVersion));
            Assert.That(snapshot.tools, Is.Not.Null);
            Assert.That(snapshot.generatedAtUtc, Is.Not.Empty);
        }
    }
}
