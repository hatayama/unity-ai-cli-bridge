using System;

namespace Hatayama.UnityAiCliBridge
{
    [Serializable]
    public sealed class BridgeDiagnosticsSnapshot
    {
        public int schemaVersion;
        public string projectPath;
        public string unityVersion;
        public string packageName;
        public string packageVersion;
        public string settingsPath;
        public bool bridgeEnabled;
        public bool bridgeRunning;
        public int activeClientCount;
        public string[] activeIdentityKeys;
        public int toolCount;
        public string[] toolNames;
        public string generatedAtUtc;
    }

    [Serializable]
    public sealed class ToolSnapshot
    {
        public int schemaVersion;
        public string generatedAtUtc;
        public ToolSnapshotEntry[] tools;
    }

    [Serializable]
    public sealed class ToolSnapshotEntry
    {
        public string name;
        public string title;
        public string description;
    }
}
