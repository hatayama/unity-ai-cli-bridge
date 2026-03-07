#!/bin/sh

set -eu

BIN_PATH="${UNITY_AI_CLI_BIN:-/tmp/unity-ai-cli}"
PROJECT_PATH="${UNITY_AI_CLI_PROJECT:-$(pwd)}"
TOOL_NAME="${UNITY_AI_CLI_E2E_TOOL:-Unity_GetConsoleLogs}"
TOOL_ARGS="${UNITY_AI_CLI_E2E_ARGS:-{\"maxEntries\":2,\"includeStackTrace\":false}}"

require_command() {
    command_name="$1"
    if command -v "$command_name" >/dev/null 2>&1; then
        return 0
    fi

    printf '%s\n' "missing required command: $command_name" >&2
    exit 1
}

require_file() {
    file_path="$1"
    if [ -f "$file_path" ]; then
        return 0
    fi

    printf '%s\n' "missing required file: $file_path" >&2
    exit 1
}

run_step() {
    step_name="$1"
    shift

    printf '%s\n' "== $step_name =="
    if ! "$@"; then
        printf '%s\n' "$step_name failed" >&2
        exit 1
    fi
    printf '\n'
}

require_file "$BIN_PATH"
require_command mktemp
require_command grep
require_command wc
require_command tr

run_step "status" "$BIN_PATH" status --project "$PROJECT_PATH" --json
run_step "doctor" "$BIN_PATH" doctor --project "$PROJECT_PATH" --json
run_step "wait" "$BIN_PATH" wait --project "$PROJECT_PATH" --for=bridge
run_step "tools" "$BIN_PATH" tools --project "$PROJECT_PATH" --json
run_step "help list" "$BIN_PATH" help --project "$PROJECT_PATH" --json
run_step "help detail" "$BIN_PATH" help "$TOOL_NAME" --project "$PROJECT_PATH" --json
run_step "describe" "$BIN_PATH" describe "$TOOL_NAME" --project "$PROJECT_PATH" --json
run_step "recipes list" "$BIN_PATH" recipes --project "$PROJECT_PATH" --json
run_step "recipe detail" "$BIN_PATH" recipes inspect-console --project "$PROJECT_PATH" --json
run_step "call" "$BIN_PATH" call "$TOOL_NAME" --project "$PROJECT_PATH" --json-args "$TOOL_ARGS"

request_file="$(mktemp)"
response_file="$(mktemp)"

cleanup() {
    rm -f "$request_file" "$response_file"
}

trap cleanup EXIT INT TERM

: >"$request_file"

write_mcp_request() {
    payload="$1"
    payload_length="$(printf '%s' "$payload" | wc -c | tr -d '[:space:]')"
    printf 'Content-Length: %s\r\n\r\n%s' "$payload_length" "$payload" >>"$request_file"
}

write_mcp_request '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}'
write_mcp_request '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
write_mcp_request "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"$TOOL_NAME\",\"arguments\":{\"maxEntries\":2,\"includeStackTrace\":false}}}"

printf '%s\n' "== mcp serve =="
if ! "$BIN_PATH" mcp serve --project "$PROJECT_PATH" <"$request_file" >"$response_file"; then
    printf '%s\n' "mcp serve failed" >&2
    exit 1
fi

cat "$response_file"

if ! grep -q '"method":"initialize"' "$request_file"; then
    printf '%s\n' "mcp serve request preparation failed" >&2
    exit 1
fi

if ! grep -q '"tools"' "$response_file"; then
    printf '%s\n' "mcp serve response does not include tools" >&2
    exit 1
fi

if ! grep -q '"structuredContent"' "$response_file"; then
    printf '%s\n' "mcp serve response does not include structuredContent" >&2
    exit 1
fi

printf '\n'
printf '%s\n' "live E2E completed successfully"
