using System.IO;

namespace Hatayama.UnityAiCliBridge.Editor
{
    public static class BridgePackagePaths
    {
        public static string ProjectRootPath
        {
            get
            {
                DirectoryInfo parentDirectory = Directory.GetParent(UnityEngine.Application.dataPath);
                System.Diagnostics.Debug.Assert(parentDirectory != null, "Project root must be resolvable from Application.dataPath");
                return parentDirectory.FullName;
            }
        }

        public static string SnapshotDirectoryPath =>
            Path.Combine(ProjectRootPath, "Library", BridgePackageConstants.SnapshotDirectoryName);

        public static string DiagnosticsFilePath =>
            Path.Combine(SnapshotDirectoryPath, BridgePackageConstants.DiagnosticsFileName);

        public static string ToolsFilePath =>
            Path.Combine(SnapshotDirectoryPath, BridgePackageConstants.ToolsFileName);
    }
}
