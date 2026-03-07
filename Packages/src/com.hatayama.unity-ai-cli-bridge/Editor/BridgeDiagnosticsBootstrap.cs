using UnityEditor;

namespace Hatayama.UnityAiCliBridge.Editor
{
    [InitializeOnLoad]
    internal static class BridgeDiagnosticsBootstrap
    {
        static BridgeDiagnosticsBootstrap()
        {
            EditorApplication.delayCall += RefreshAfterLoad;
            AssemblyReloadEvents.afterAssemblyReload += RefreshAfterAssemblyReload;
        }

        static void RefreshAfterLoad()
        {
            BridgeDiagnosticsSnapshotWriter.RefreshSnapshots();
        }

        static void RefreshAfterAssemblyReload()
        {
            BridgeDiagnosticsSnapshotWriter.RefreshSnapshots();
        }
    }
}
