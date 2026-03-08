package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	liveE2EEnvVar          = "UNITY_AI_CLI_E2E"
	liveE2EBinaryEnvVar    = "UNITY_AI_CLI_E2E_BIN"
	defaultLiveE2ETool     = "Unity_GetConsoleLogs"
	defaultLiveE2EJSONArgs = `{"maxEntries":2,"includeStackTrace":false}`
	defaultLiveE2EBinary   = "/tmp/unity-ai-cli"
	knownLiveToolCount     = 51
	smokeScriptPath        = "Assets/TutorialInfo/Scripts/Readme.cs"
	smokeScriptURI         = "unity://path/Assets/TutorialInfo/Scripts/Readme.cs"
	smokeInstalledPackage  = "com.unity.ai.assistant"
)

const liveToolCallTimeout = 30 * time.Second

type liveToolSmokeExpectation string

const (
	smokeExpectSuccess         liveToolSmokeExpectation = "success"
	smokeExpectStructuredError liveToolSmokeExpectation = "structured_error"
	smokeExpectAnyStructured   liveToolSmokeExpectation = "any_structured"
)

type liveToolSmokeCase struct {
	Arguments   map[string]any
	Expectation liveToolSmokeExpectation
	SkipReason  string
	Timeout     time.Duration
}

func TestLiveUnityEndToEnd(t *testing.T) {
	if strings.TrimSpace(liveE2EEnvValue(t, liveE2EEnvVar)) == "" {
		t.Skipf("%s is not set", liveE2EEnvVar)
	}

	toolName := defaultLiveE2ETool
	binaryPath := resolveLiveBinaryPath(t)

	statusOutput := runCLICommand(t, binaryPath, nil, "status", "--json")
	statusPayload := decodeJSONObject(t, statusOutput)
	connectionPayload := decodeObjectField(t, statusPayload, "connection")
	connectionPath := stringField(t, connectionPayload, "connection_path")
	if strings.TrimSpace(connectionPath) == "" {
		t.Fatal("expected status output to include connection_path")
	}

	doctorOutput := runCLICommand(t, binaryPath, nil, "doctor", "--json")
	doctorPayload := decodeJSONObject(t, doctorOutput)
	if _, ok := doctorPayload["checks"]; !ok {
		t.Fatalf("expected doctor output to include checks, got %#v", doctorPayload)
	}

	waitOutput := runCLICommand(t, binaryPath, nil, "wait", "--for=bridge")
	if !strings.Contains(string(waitOutput), "bridge") {
		t.Fatalf("expected wait output to mention bridge, got %q", string(waitOutput))
	}

	listOutput := runCLICommand(t, binaryPath, nil, "tools", "--json")
	listPayload := decodeJSONObject(t, listOutput)
	toolsValue, ok := listPayload["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %#v", listPayload["tools"])
	}

	foundTool := false
	for _, rawTool := range toolsValue {
		toolPayload, ok := rawTool.(map[string]any)
		if !ok {
			t.Fatalf("expected tool object, got %#v", rawTool)
		}

		if stringField(t, toolPayload, "name") == toolName {
			foundTool = true
			break
		}
	}

	if !foundTool {
		t.Fatalf("expected to find tool %s in %#v", toolName, toolsValue)
	}

	helpListOutput := runCLICommand(t, binaryPath, nil, "help", "--json")
	helpListPayload := decodeJSONObject(t, helpListOutput)
	helpToolsValue := decodeArrayField(t, helpListPayload, "tools")
	if len(helpToolsValue) == 0 {
		t.Fatal("expected help output to include enabled tools")
	}

	helpOutput := runCLICommand(t, binaryPath, nil, "help", toolName, "--json")
	helpPayload := decodeJSONObject(t, helpOutput)
	if stringField(t, helpPayload, "name") != toolName {
		t.Fatalf("expected help output for %s, got %#v", toolName, helpPayload)
	}

	describeOutput := runCLICommand(t, binaryPath, nil, "describe", toolName, "--json")
	describePayload := decodeJSONObject(t, describeOutput)
	if stringField(t, describePayload, "name") != toolName {
		t.Fatalf("expected describe output for %s, got %#v", toolName, describePayload)
	}

	recipesOutput := runCLICommand(t, binaryPath, nil, "recipes", "--json")
	recipesPayload := decodeJSONObject(t, recipesOutput)
	recipeList := decodeArrayField(t, recipesPayload, "recipes")
	if len(recipeList) == 0 {
		t.Fatal("expected recipes output to include at least one recipe")
	}

	recipeOutput := runCLICommand(t, binaryPath, nil, "recipes", "inspect-console", "--json")
	recipePayload := decodeJSONObject(t, recipeOutput)
	if stringField(t, recipePayload, "id") != "inspect-console" {
		t.Fatalf("expected inspect-console recipe, got %#v", recipePayload)
	}

	callOutput := runCLICommand(t, binaryPath, nil, "call", toolName, "--json-args", defaultLiveE2EJSONArgs)
	callPayload := decodeJSONObject(t, callOutput)
	successValue, ok := callPayload["success"].(bool)
	if !ok {
		t.Fatalf("expected success field, got %#v", callPayload["success"])
	}

	if !successValue {
		t.Fatalf("expected successful tool call, got %#v", callPayload)
	}

	dataPayload := decodeObjectField(t, callPayload, "data")
	if _, ok := dataPayload["logs"]; !ok {
		t.Fatalf("expected console log payload, got %#v", dataPayload)
	}

	mcpInput := bytes.NewBuffer(nil)
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
		},
	})
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": toolName,
			"arguments": map[string]any{
				"maxEntries":        2,
				"includeStackTrace": false,
			},
		},
	})

	mcpOutput := runCLICommand(t, binaryPath, mcpInput.Bytes(), "mcp", "serve")
	mcpResponses := readRPCResponses(t, mcpOutput)
	if len(mcpResponses) != 3 {
		t.Fatalf("expected 3 MCP responses, got %d", len(mcpResponses))
	}

	mcpToolsResult := decodeArrayField(t, decodeObjectField(t, mcpResponses[1], "result"), "tools")
	if len(mcpToolsResult) == 0 {
		t.Fatal("expected MCP tools/list to return at least one tool")
	}

	mcpCallResult := decodeObjectField(t, mcpResponses[2], "result")
	structuredContent := decodeObjectField(t, mcpCallResult, "structuredContent")
	structuredSuccess, ok := structuredContent["success"].(bool)
	if !ok {
		t.Fatalf("expected structuredContent.success, got %#v", structuredContent["success"])
	}

	if !structuredSuccess {
		t.Fatalf("expected successful MCP tool call, got %#v", structuredContent)
	}
}

func TestLiveUnityAllToolsCallableViaMCP(t *testing.T) {
	if strings.TrimSpace(liveE2EEnvValue(t, liveE2EEnvVar)) == "" {
		t.Skipf("%s is not set", liveE2EEnvVar)
	}

	binaryPath := resolveLiveBinaryPath(t)
	smokeCases := liveToolSmokeCases()
	assertLiveSmokeCatalogCoverage(t, smokeCases)

	listPayload := decodeJSONObject(t, runCLICommand(t, binaryPath, nil, "tools", "--json"))
	cliToolNames := toolNamesFromArray(t, decodeArrayField(t, listPayload, "tools"))
	t.Logf("live catalog exposes %d tools", len(cliToolNames))
	assertToolNameSetsEqual(t, knownLiveToolNames, cliToolNames)

	mcpToolsResponse := listToolsViaMCP(t, binaryPath)
	assertNoRPCError(t, mcpToolsResponse)
	mcpToolNames := toolNamesFromArray(t, decodeArrayField(t, decodeObjectField(t, mcpToolsResponse, "result"), "tools"))
	assertToolNameSetsEqual(t, knownLiveToolNames, mcpToolNames)
	assertToolNameSetsEqual(t, cliToolNames, mcpToolNames)

	for _, toolName := range cliToolNames {
		smokeCase, ok := smokeCases[toolName]
		if !ok {
			t.Fatalf("missing smoke case for live tool %s", toolName)
		}

		t.Run(toolName, func(t *testing.T) {
			if smokeCase.SkipReason != "" {
				t.Skip(smokeCase.SkipReason)
			}

			response := callToolViaMCP(t, binaryPath, toolName, smokeCase)
			assertLiveToolSmokeResponse(t, toolName, smokeCase, response)
		})
	}
}

func liveE2EEnvValue(t *testing.T, envVar string) string {
	t.Helper()
	return strings.TrimSpace(os.Getenv(envVar))
}

func resolveLiveBinaryPath(t *testing.T) string {
	t.Helper()

	binaryPath := strings.TrimSpace(os.Getenv(liveE2EBinaryEnvVar))
	if binaryPath == "" {
		binaryPath = defaultLiveE2EBinary
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("failed to access E2E binary %s: %v", binaryPath, err)
	}

	if info.IsDir() {
		t.Fatalf("expected E2E binary path, got directory %s", binaryPath)
	}

	return binaryPath
}

func runCLICommand(t *testing.T, binaryPath string, stdin []byte, args ...string) []byte {
	t.Helper()
	return runCLICommandWithTimeout(t, binaryPath, stdin, 45*time.Second, args...)
}

func runCLICommandWithTimeout(
	t *testing.T,
	binaryPath string,
	stdin []byte,
	timeout time.Duration,
	args ...string,
) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, binaryPath, args...)
	command.Stdin = bytes.NewReader(stdin)

	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	command.Stdout = stdout
	command.Stderr = stderr

	if err := command.Run(); err != nil {
		t.Fatalf("command %q failed: %v: %s", strings.Join(append([]string{binaryPath}, args...), " "), err, stderr.String())
	}

	return stdout.Bytes()
}

func assertLiveSmokeCatalogCoverage(t *testing.T, smokeCases map[string]liveToolSmokeCase) {
	t.Helper()

	if len(knownLiveToolNames) != knownLiveToolCount {
		t.Fatalf("expected %d known live tools, got %d", knownLiveToolCount, len(knownLiveToolNames))
	}

	if len(smokeCases) != len(knownLiveToolNames) {
		t.Fatalf("expected %d smoke cases, got %d", len(knownLiveToolNames), len(smokeCases))
	}

	expectedTools := make(map[string]struct{}, len(knownLiveToolNames))
	for _, toolName := range knownLiveToolNames {
		if _, exists := expectedTools[toolName]; exists {
			t.Fatalf("duplicate known live tool name: %s", toolName)
		}

		expectedTools[toolName] = struct{}{}
		if _, ok := smokeCases[toolName]; !ok {
			t.Fatalf("missing smoke case for known tool %s", toolName)
		}
	}

	for toolName := range smokeCases {
		if _, ok := expectedTools[toolName]; !ok {
			t.Fatalf("unexpected smoke case for unknown tool %s", toolName)
		}
	}
}

func decodeJSONObject(t *testing.T, payload []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to decode JSON object %q: %v", string(payload), err)
	}

	return decoded
}

func decodeObjectField(t *testing.T, payload map[string]any, fieldName string) map[string]any {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	decoded, ok := fieldValue.(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object, got %#v", fieldName, fieldValue)
	}

	return decoded
}

func stringField(t *testing.T, payload map[string]any, fieldName string) string {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	stringValue, ok := fieldValue.(string)
	if !ok {
		t.Fatalf("expected %s to be a string, got %#v", fieldName, fieldValue)
	}

	return stringValue
}

func decodeArrayField(t *testing.T, payload map[string]any, fieldName string) []any {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	arrayValue, ok := fieldValue.([]any)
	if !ok {
		t.Fatalf("expected %s to be an array, got %#v", fieldName, fieldValue)
	}

	return arrayValue
}

func toolNamesFromArray(t *testing.T, tools []any) []string {
	t.Helper()

	names := make([]string, 0, len(tools))
	for _, rawTool := range tools {
		toolPayload, ok := rawTool.(map[string]any)
		if !ok {
			t.Fatalf("expected tool object, got %#v", rawTool)
		}

		names = append(names, stringField(t, toolPayload, "name"))
	}

	return names
}

func listToolsViaMCP(t *testing.T, binaryPath string) map[string]any {
	t.Helper()

	mcpInput := bytes.NewBuffer(nil)
	writeInitializeRequest(t, mcpInput, 1)
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]any{},
	})

	responses := readRPCResponses(
		t,
		runCLICommandWithTimeout(t, binaryPath, mcpInput.Bytes(), 45*time.Second, "mcp", "serve"),
	)

	responsesByID := indexResponsesByID(t, responses)
	response, ok := responsesByID[2]
	if !ok {
		t.Fatal("expected MCP tools/list response with id 2")
	}

	return response
}

func callToolViaMCP(t *testing.T, binaryPath string, toolName string, smokeCase liveToolSmokeCase) map[string]any {
	t.Helper()

	mcpInput := bytes.NewBuffer(nil)
	writeInitializeRequest(t, mcpInput, 1)
	writeRPCRequest(t, mcpInput, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": smokeCase.Arguments,
		},
	})

	timeout := liveToolCallTimeout
	if smokeCase.Timeout > 0 {
		timeout = smokeCase.Timeout
	}

	responses := readRPCResponses(
		t,
		runCLICommandWithTimeout(t, binaryPath, mcpInput.Bytes(), timeout, "mcp", "serve"),
	)

	responsesByID := indexResponsesByID(t, responses)
	response, ok := responsesByID[2]
	if !ok {
		t.Fatalf("expected tools/call response for %s with id 2", toolName)
	}

	return response
}

func indexResponsesByID(t *testing.T, responses []map[string]any) map[int]map[string]any {
	t.Helper()

	indexed := make(map[int]map[string]any, len(responses))
	for _, response := range responses {
		requestID := intIDField(t, response, "id")
		indexed[requestID] = response
	}

	return indexed
}

func intIDField(t *testing.T, payload map[string]any, fieldName string) int {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok {
		t.Fatalf("expected field %s in %#v", fieldName, payload)
	}

	numberValue, ok := fieldValue.(float64)
	if !ok {
		t.Fatalf("expected %s to be numeric, got %#v", fieldName, fieldValue)
	}

	return int(numberValue)
}

func assertNoRPCError(t *testing.T, payload map[string]any) {
	t.Helper()

	if errorValue, ok := payload["error"]; ok && errorValue != nil {
		t.Fatalf("expected RPC success payload, got error %#v", payload["error"])
	}
}

func assertToolNameSetsEqual(t *testing.T, expected []string, actual []string) {
	t.Helper()

	expectedNames := append([]string(nil), expected...)
	actualNames := append([]string(nil), actual...)
	sort.Strings(expectedNames)
	sort.Strings(actualNames)

	if len(expectedNames) != len(actualNames) {
		t.Fatalf("expected %d tools, got %d", len(expectedNames), len(actualNames))
	}

	for index, expectedName := range expectedNames {
		if actualNames[index] != expectedName {
			t.Fatalf("expected tool set %#v, got %#v", expectedNames, actualNames)
		}
	}
}

func assertLiveToolSmokeResponse(
	t *testing.T,
	toolName string,
	smokeCase liveToolSmokeCase,
	response map[string]any,
) {
	t.Helper()

	assertNoRPCError(t, response)

	resultPayload := decodeObjectField(t, response, "result")
	contentPayload := decodeArrayField(t, resultPayload, "content")
	if len(contentPayload) == 0 {
		t.Fatalf("expected content for %s, got %#v", toolName, resultPayload)
	}

	firstContent, ok := contentPayload[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first content item to be an object, got %#v", contentPayload[0])
	}

	textValue, ok := firstContent["text"].(string)
	if !ok {
		t.Fatalf("expected content text for %s, got %#v", toolName, firstContent["text"])
	}

	if strings.TrimSpace(textValue) == "" {
		t.Fatalf("expected non-empty content text for %s, got %#v", toolName, firstContent)
	}

	structuredFailure := resultIndicatesStructuredFailure(t, resultPayload)
	switch smokeCase.Expectation {
	case smokeExpectSuccess:
		structuredContent, ok := optionalObjectField(t, resultPayload, "structuredContent")
		if !ok {
			t.Fatalf("expected structured success payload for %s, got %#v", toolName, resultPayload)
		}

		success, hasSuccess := optionalBoolField(t, structuredContent, "success")
		if hasSuccess && !success {
			t.Fatalf("expected successful structured payload for %s, got %#v", toolName, resultPayload)
		}

		if structuredFailure {
			t.Fatalf("expected success response for %s, got %#v", toolName, resultPayload)
		}
	case smokeExpectStructuredError:
		if !structuredFailure {
			t.Fatalf("expected structured error response for %s, got %#v", toolName, resultPayload)
		}
	case smokeExpectAnyStructured:
		structuredContent, ok := optionalObjectField(t, resultPayload, "structuredContent")
		if !ok || len(structuredContent) == 0 {
			t.Fatalf("expected structured payload for %s, got %#v", toolName, resultPayload)
		}
	default:
		t.Fatalf("unexpected smoke expectation %q for %s", smokeCase.Expectation, toolName)
	}
}

func resultIndicatesStructuredFailure(t *testing.T, payload map[string]any) bool {
	t.Helper()

	isError, hasIsError := optionalBoolField(t, payload, "isError")
	if hasIsError {
		return isError
	}

	structuredContentValue, ok := payload["structuredContent"]
	if !ok || structuredContentValue == nil {
		return false
	}

	structuredContent, ok := structuredContentValue.(map[string]any)
	if !ok {
		t.Fatalf("expected structuredContent to be an object, got %#v", structuredContentValue)
	}

	success, hasSuccess := optionalBoolField(t, structuredContent, "success")
	if !hasSuccess {
		return false
	}

	return !success
}

func optionalBoolField(t *testing.T, payload map[string]any, fieldName string) (bool, bool) {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok || fieldValue == nil {
		return false, false
	}

	boolValue, ok := fieldValue.(bool)
	if !ok {
		t.Fatalf("expected %s to be a bool, got %#v", fieldName, fieldValue)
	}

	return boolValue, true
}

func optionalObjectField(t *testing.T, payload map[string]any, fieldName string) (map[string]any, bool) {
	t.Helper()

	fieldValue, ok := payload[fieldName]
	if !ok || fieldValue == nil {
		return nil, false
	}

	objectValue, ok := fieldValue.(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be an object, got %#v", fieldName, fieldValue)
	}

	return objectValue, true
}

func writeInitializeRequest(t *testing.T, buffer *bytes.Buffer, requestID int) {
	t.Helper()

	writeRPCRequest(t, buffer, map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
		},
	})
}

func writeRPCRequest(t *testing.T, buffer *bytes.Buffer, payload map[string]any) {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to encode RPC payload: %v", err)
	}

	if _, err := fmt.Fprintf(buffer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		t.Fatalf("failed to write RPC header: %v", err)
	}

	if _, err := buffer.Write(body); err != nil {
		t.Fatalf("failed to write RPC body: %v", err)
	}
}

func readRPCResponses(t *testing.T, payload []byte) []map[string]any {
	t.Helper()

	reader := bytes.NewBuffer(payload)
	responses := make([]map[string]any, 0, 3)
	for reader.Len() > 0 {
		header, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read RPC header: %v", err)
		}

		if header == "\r\n" {
			continue
		}

		var contentLength int
		if _, err := fmt.Sscanf(header, "Content-Length: %d\r\n", &contentLength); err != nil {
			t.Fatalf("failed to parse Content-Length from %q: %v", header, err)
		}

		separator := make([]byte, 2)
		if _, err := reader.Read(separator); err != nil {
			t.Fatalf("failed to read RPC separator: %v", err)
		}

		body := make([]byte, contentLength)
		if _, err := reader.Read(body); err != nil {
			t.Fatalf("failed to read RPC body: %v", err)
		}

		responses = append(responses, decodeJSONObject(t, body))
	}

	return responses
}

var knownLiveToolNames = []string{
	"Unity_ApplyTextEdits",
	"Unity_AssetGeneration_ConvertSpriteSheetToAnimationClip",
	"Unity_AssetGeneration_ConvertToMaterial",
	"Unity_AssetGeneration_ConvertToTerrainLayer",
	"Unity_AssetGeneration_CreateAnimatorControllerFromClip",
	"Unity_AssetGeneration_EditAnimationClipTool",
	"Unity_AssetGeneration_GenerateAsset",
	"Unity_AssetGeneration_GetCompositionPatterns",
	"Unity_AssetGeneration_GetModels",
	"Unity_AssetGeneration_ManageInterrupted",
	"Unity_AudioClip_Edit",
	"Unity_Camera_Capture",
	"Unity_CreateScript",
	"Unity_DeleteScript",
	"Unity_EditorWindow_CaptureScreenshot",
	"Unity_FindInFile",
	"Unity_FindProjectAssets",
	"Unity_GetConsoleLogs",
	"Unity_GetProjectData",
	"Unity_GetSha",
	"Unity_GetUserGuidelines",
	"Unity_ImportExternalModel",
	"Unity_ListResources",
	"Unity_ManageAsset",
	"Unity_ManageEditor",
	"Unity_ManageGameObject",
	"Unity_ManageMenuItem",
	"Unity_ManageScene",
	"Unity_ManageScript",
	"Unity_ManageScript_capabilities",
	"Unity_ManageShader",
	"Unity_PackageManager_ExecuteAction",
	"Unity_PackageManager_GetData",
	"Unity_Profiler_GetBottomUpSampleTimeSummary",
	"Unity_Profiler_GetFrameGcAllocationsSummary",
	"Unity_Profiler_GetFrameRangeGcAllocationsSummary",
	"Unity_Profiler_GetFrameRangeTopTimeSummary",
	"Unity_Profiler_GetFrameSelfTimeSamplesSummary",
	"Unity_Profiler_GetFrameTopTimeSamplesSummary",
	"Unity_Profiler_GetOverallGcAllocationsSummary",
	"Unity_Profiler_GetRelatedSamplesTimeSummary",
	"Unity_Profiler_GetSampleGcAllocationSummary",
	"Unity_Profiler_GetSampleGcAllocationSummaryByMarkerPath",
	"Unity_Profiler_GetSampleTimeSummary",
	"Unity_Profiler_GetSampleTimeSummaryByMarkerPath",
	"Unity_ReadConsole",
	"Unity_ReadResource",
	"Unity_RunCommand",
	"Unity_SceneView_CaptureMultiAngleSceneView",
	"Unity_ScriptApplyEdits",
	"Unity_ValidateScript",
}

func liveToolSmokeCases() map[string]liveToolSmokeCase {
	return map[string]liveToolSmokeCase{
		"Unity_ApplyTextEdits": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_ConvertSpriteSheetToAnimationClip": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_ConvertToMaterial": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_ConvertToTerrainLayer": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_CreateAnimatorControllerFromClip": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_EditAnimationClipTool": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_GenerateAsset": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_GetCompositionPatterns": {
			Expectation: smokeExpectSuccess,
			Arguments:   map[string]any{},
		},
		"Unity_AssetGeneration_GetModels": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"includeAllModels": false,
			},
			SkipReason: "known live issue: Unity asset-generation backend does not return within the CLI timeout in this environment",
		},
		"Unity_AssetGeneration_ManageInterrupted": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"command": "List",
			},
		},
		"Unity_AudioClip_Edit": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_Camera_Capture": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"cameraInstanceID": -1,
			},
		},
		"Unity_CreateScript": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_DeleteScript": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_EditorWindow_CaptureScreenshot": {
			Expectation: smokeExpectAnyStructured,
			Arguments:   map[string]any{},
		},
		"Unity_FindInFile": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Uri":        smokeScriptURI,
				"Pattern":    "class",
				"IgnoreCase": true,
				"MaxResults": 5,
			},
		},
		"Unity_FindProjectAssets": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"query":      "readme",
				"startIndex": 0,
			},
		},
		"Unity_GetConsoleLogs": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"maxEntries":        5,
				"includeStackTrace": false,
				"logTypes":          "Log",
			},
		},
		"Unity_GetProjectData": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"maxSceneDepth":    1,
				"maxTaxonomyDepth": 1,
				"maxAssetItems":    5,
				"maxOutputChars":   4000,
			},
		},
		"Unity_GetSha": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Uri": smokeScriptPath,
			},
		},
		"Unity_GetUserGuidelines": {
			Expectation: smokeExpectSuccess,
			Arguments:   map[string]any{},
		},
		"Unity_ImportExternalModel": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_ListResources": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Under":   "Assets",
				"Pattern": "*.cs",
				"Limit":   10,
			},
		},
		"Unity_ManageAsset": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Action":        "Search",
				"Path":          "Assets",
				"SearchPattern": "*.cs",
				"PageSize":      5,
				"PageNumber":    1,
			},
		},
		"Unity_ManageEditor": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Action": "GetState",
			},
		},
		"Unity_ManageGameObject": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_ManageMenuItem": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Action":  "List",
				"Search":  "File/Save",
				"Refresh": false,
			},
		},
		"Unity_ManageScene": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Action": "GetActive",
			},
		},
		"Unity_ManageScript": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_ManageScript_capabilities": {
			Expectation: smokeExpectSuccess,
			Arguments:   map[string]any{},
		},
		"Unity_ManageShader": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_PackageManager_ExecuteAction": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_PackageManager_GetData": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"packageID":     smokeInstalledPackage,
				"installedOnly": true,
			},
		},
		"Unity_Profiler_GetBottomUpSampleTimeSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":          -1,
				"threadName":          "Main Thread",
				"bottomUpSampleIndex": -1,
			},
		},
		"Unity_Profiler_GetFrameGcAllocationsSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex": -1,
			},
		},
		"Unity_Profiler_GetFrameRangeGcAllocationsSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"startFrameIndex": -1,
				"lastFrameIndex":  -1,
			},
		},
		"Unity_Profiler_GetFrameRangeTopTimeSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"startFrameIndex": -1,
				"lastFrameIndex":  -1,
				"targetFrameTime": 16.7,
			},
		},
		"Unity_Profiler_GetFrameSelfTimeSamplesSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex": -1,
			},
		},
		"Unity_Profiler_GetFrameTopTimeSamplesSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":      -1,
				"targetFrameTime": 16.7,
			},
		},
		"Unity_Profiler_GetOverallGcAllocationsSummary": {
			Expectation: smokeExpectAnyStructured,
			Arguments:   map[string]any{},
		},
		"Unity_Profiler_GetRelatedSamplesTimeSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":        -1,
				"threadName":        "Main Thread",
				"sampleIndex":       -1,
				"relatedThreadName": "Render Thread",
			},
		},
		"Unity_Profiler_GetSampleGcAllocationSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":  -1,
				"threadName":  "Main Thread",
				"sampleIndex": -1,
			},
		},
		"Unity_Profiler_GetSampleGcAllocationSummaryByMarkerPath": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":   -1,
				"threadName":   "Main Thread",
				"markerIdPath": "Invalid/Marker",
			},
		},
		"Unity_Profiler_GetSampleTimeSummary": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":  -1,
				"threadName":  "Main Thread",
				"sampleIndex": -1,
			},
		},
		"Unity_Profiler_GetSampleTimeSummaryByMarkerPath": {
			Expectation: smokeExpectStructuredError,
			Arguments: map[string]any{
				"frameIndex":   -1,
				"threadName":   "Main Thread",
				"markerIdPath": "Invalid/Marker",
			},
		},
		"Unity_ReadConsole": {
			Expectation: smokeExpectAnyStructured,
			Arguments: map[string]any{
				"Action":            "Get",
				"Types":             []string{"Log"},
				"Count":             5,
				"Format":            "Json",
				"IncludeStacktrace": false,
			},
		},
		"Unity_ReadResource": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Uri":       smokeScriptURI,
				"StartLine": 1,
				"LineCount": 5,
			},
		},
		"Unity_RunCommand": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_SceneView_CaptureMultiAngleSceneView": {
			Expectation: smokeExpectAnyStructured,
			Arguments:   map[string]any{},
		},
		"Unity_ScriptApplyEdits": {
			Expectation: smokeExpectStructuredError,
			Arguments:   map[string]any{},
		},
		"Unity_ValidateScript": {
			Expectation: smokeExpectSuccess,
			Arguments: map[string]any{
				"Uri":                smokeScriptPath,
				"Level":              "basic",
				"IncludeDiagnostics": false,
			},
		},
	}
}
