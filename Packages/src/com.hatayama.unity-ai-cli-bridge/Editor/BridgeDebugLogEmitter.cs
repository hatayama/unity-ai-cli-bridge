using System;
using UnityEngine;

namespace Hatayama.UnityAiCliBridge.Editor
{
    internal static class BridgeDebugLogEmitter
    {
        public static void EmitSampleLogs()
        {
            string timestamp = DateTime.UtcNow.ToString("O");

            Debug.Log($"unity-ai-cli sample info log {timestamp}");
            Debug.LogWarning($"unity-ai-cli sample warning log {timestamp}");
            Debug.LogError($"unity-ai-cli sample error log {timestamp}");
        }
    }
}
